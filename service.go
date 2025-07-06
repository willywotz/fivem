package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

var (
	svcName        = "FiveMTools"
	svcDisplayName = "FiveM Tools"
)

var elog debug.Log

type exampleService struct{}

func (m *exampleService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	updateTicker := time.NewTicker(15 * time.Minute)
	defer updateTicker.Stop()

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	_ = elog.Info(1, fmt.Sprintf("Service (Version: %s) started.", buildVersion))

loop:
	for {
		select {
		case <-updateTicker.C:
			if err := handleAutoUpdate(); err != nil {
				_ = elog.Error(1, fmt.Sprintf("auto update failed: %v", err))
			}
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
				// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
				time.Sleep(100 * time.Millisecond)
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				break loop
			default:
				_ = elog.Error(1, fmt.Sprintf("unexpected control request #%d", c))
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}

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

	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	err = run(name, &exampleService{})
	if err != nil {
		_ = elog.Error(1, fmt.Sprintf("%s service failed: %v", name, err))
		return
	}
}

func GetServiceDirectory(name string) (string, error) {
	programDataDir := os.Getenv("ProgramData")
	if programDataDir == "" {
		return "", fmt.Errorf("PROGRAMDATA environment variable not set")
	}
	targetDir := filepath.Join(programDataDir, name)
	if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create service directory in ProgramData: %w", err)
	}
	return targetDir, nil
}

func installService(name, displayName string) error {
	srcPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get source executable path: %w", err)
	}

	targetDir, err := GetServiceDirectory(name)
	if err != nil {
		return fmt.Errorf("failed to get service directory: %w", err)
	}

	targetPath := filepath.Join(targetDir, filepath.Base(srcPath))

	if !strings.HasPrefix(srcPath, targetDir) {
		if err := copyFile(srcPath, targetDir); err != nil {
			return fmt.Errorf("failed to copy executable to ProgramData: %w", err)
		}
	}

	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() { _ = m.Disconnect() }()
	s, err := m.OpenService(name)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists", name)
	}
	s, err = m.CreateService(name, targetPath, mgr.Config{
		DisplayName: displayName,
		StartType:   mgr.StartAutomatic,
	})
	if err != nil {
		return err
	}
	defer s.Close()
	err = s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},  // Restart after 5 seconds on 1st failure
		{Type: mgr.ServiceRestart, Delay: 10 * time.Second}, // Restart after 10 seconds on 2nd failure
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second}, // Restart after 60 seconds on subsequent failures
	}, 10000)
	if err != nil {
		_ = s.Delete()
		return fmt.Errorf("failed to set service recovery actions: %w", err)
	}
	err = eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		_ = s.Delete()
		return fmt.Errorf("SetupEventLogSource() failed: %s", err)
	}
	return nil
}

func removeService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() { _ = m.Disconnect() }()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("service %s is not installed", name)
	}
	defer s.Close()
	if status, err := s.Query(); err != nil {
		return fmt.Errorf("could not retrieve service status: %v", err)
	} else if status.State != svc.Stopped {
		if err := controlService(name, svc.Stop, svc.Stopped); err != nil {
			return fmt.Errorf("could not stop service: %v", err)
		}
	}
	err = s.Delete()
	if err != nil {
		return err
	}
	err = eventlog.Remove(name)
	if err != nil {
		return fmt.Errorf("RemoveEventLogSource() failed: %s", err)
	}
	return nil
}

func startService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() { _ = m.Disconnect() }()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	if status, err := s.Query(); err != nil {
		return fmt.Errorf("could not retrieve service status: %v", err)
	} else if status.State == svc.Running {
		return fmt.Errorf("service %s is already running", name)
	} else if status.State != svc.Stopped {
		if err := controlService(name, svc.Stop, svc.Stopped); err != nil {
			return fmt.Errorf("could not stop service: %v", err)
		}
	}
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
	defer func() { _ = m.Disconnect() }()
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
