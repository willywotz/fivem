package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

var elog debug.Log

var svcName = "FiveM Service"

func runService(name string, isDebug bool) {
	var err error
	if isDebug {
		elog = debug.New(name)
	} else {
		elog, err = eventlog.Open(name)
		if err != nil {
			return
		}
	}
	defer elog.Close()

	_ = elog.Info(1, fmt.Sprintf("starting %s service, version: %s", name, buildVersion))
	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	err = run(name, &exampleService{})
	if err != nil {
		_ = elog.Error(1, fmt.Sprintf("%s service failed: %v", name, err))
		return
	}
	_ = elog.Info(1, fmt.Sprintf("%s service stopped", name))
}

type exampleService struct{}

func (m *exampleService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue
	changes <- svc.Status{State: svc.StartPending}
	fasttick := time.Tick(500 * time.Millisecond)
	slowtick := time.Tick(2 * time.Second)
	tick := fasttick
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
loop:
	for {
		select {
		case <-tick:
			beep()
			// _ = elog.Info(1, "beep")
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
				// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
				time.Sleep(100 * time.Millisecond)
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				// golang.org/x/sys/windows/svc.TestExample is verifying this output.
				testOutput := strings.Join(args, "-")
				testOutput += fmt.Sprintf("-%d", c.Context)
				_ = elog.Info(1, testOutput)
				break loop
			case svc.Pause:
				changes <- svc.Status{State: svc.Paused, Accepts: cmdsAccepted}
				tick = slowtick
			case svc.Continue:
				changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
				tick = fasttick
			default:
				_ = elog.Error(1, fmt.Sprintf("unexpected control request #%d", c))
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}

// var beepFunc = syscall.MustLoadDLL("user32.dll").MustFindProc("MessageBeep")

func beep() {
	// _ = exec.Command("powershell", "-c", "[console]::beep(500,300)").Run()
}

func startService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() {
		_ = m.Disconnect()
	}()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	err = s.Start()
	if err != nil {
		return fmt.Errorf("could not start service: %v", err)
	}
	return nil
}

func controlService(name string, c svc.Cmd, to svc.State) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() {
		_ = m.Disconnect()
	}()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	status, err := s.Control(c)
	if err != nil {
		return fmt.Errorf("could not send control=%d: %v", c, err)
	}
	timeout := time.Now().Add(10 * time.Second)
	for status.State != to {
		if timeout.Before(time.Now()) {
			return fmt.Errorf("timeout waiting for service to go to state=%d", to)
		}
		time.Sleep(300 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return fmt.Errorf("could not retrieve service status: %v", err)
		}
	}
	return nil
}

func installService(name, desc string) error {
	exepath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	if filepath.Base(exepath) != binaryFileName {
		return fmt.Errorf("executable is already named %s", binaryFileName)
	}

	userDir, _ := os.UserConfigDir()
	rootDir := filepath.Join(userDir, "willywotz", "fivem")

	if _, err := os.Stat(rootDir); os.IsNotExist(err) {
		if err := os.MkdirAll(rootDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create root directory %s: %w", rootDir, err)
		}
	}

	if strings.HasPrefix(exepath, rootDir) {
	} else if _, err := os.Stat(filepath.Join(rootDir, binaryFileName)); err == nil {
		exepath = filepath.Join(rootDir, binaryFileName)
	} else if f, err := os.Create(filepath.Join(rootDir, binaryFileName)); err == nil {
		currentFile, err := os.Open(exepath)
		if err != nil {
			_ = f.Close()
			return fmt.Errorf("failed to open current executable: %w", err)
		}
		_, err = io.Copy(f, currentFile)
		if err != nil {
			_ = f.Close()
			_ = currentFile.Close()
			return fmt.Errorf("failed to copy current executable: %w", err)
		}
		_ = f.Sync()
		_ = f.Close()
		_ = currentFile.Close()
		exepath = filepath.Join(rootDir, binaryFileName)
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer func() {
		_ = m.Disconnect()
	}()
	s, err := m.OpenService(name)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists", name)
	}
	s, err = m.CreateService(name, exepath, mgr.Config{StartType: mgr.StartAutomatic, DisplayName: desc})
	if err != nil {
		return fmt.Errorf("failed to create service %s: %w", name, err)
	}
	defer s.Close()
	err = eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		_ = s.Delete()
		return fmt.Errorf("SetupEventLogSource() failed: %s", err)
	}
	return startService(name)
}

func removeService(name string) error {
	_ = controlService(name, svc.Stop, svc.Stopped)

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer func() {
		_ = m.Disconnect()
	}()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("service %s is not installed", name)
	}
	defer s.Close()
	err = s.Delete()
	if err != nil {
		return fmt.Errorf("RemoveService() failed: %w", err)
	}
	err = eventlog.Remove(name)
	if err != nil {
		return fmt.Errorf("RemoveEventLogSource() failed: %w", err)
	}
	return nil
}
