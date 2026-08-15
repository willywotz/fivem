# Context

## Auto updater — fully background

The auto updater runs without any visible effect on the user.

- `update()` (`update.go`) starts one background goroutine. It does the first
  check inside the goroutine, so program startup never waits for the network.
- `handleUpdate()` gives no console output. It writes only errors, through
  `failedf` (event log in the service, standard error otherwise).
- On a new version, the restart is silent. The new process starts with no
  standard handles and the `CREATE_NO_WINDOW` flag, so no console window shows.
- The Windows service path exits with `os.Exit(1)` and Service Recovery
  restarts it. Its periodic check uses one `time.NewTicker` made before the
  loop. Before, it used `time.Tick` inside the `select`, which made a new timer
  on every service control request and could starve the update check.

## Auto updater — verified behavior

- `selfupdate.UpdateSelf` (go-selfupdate v1.5.0) downloads and applies the new
  binary in one call, then returns a non-`nil` `*Release` when `err` is `nil`.
  So `release.GreaterThan(version)` is safe; it is `true` only after a real
  update, which is when the process restarts.
- The restart uses the `CREATE_NEW_PROCESS_GROUP | CREATE_NO_WINDOW` flags.
  `DETACHED_PROCESS` is not used, because Windows ignores `CREATE_NO_WINDOW`
  when it is joined with `DETACHED_PROCESS`.

## Windows service — verified behavior

- `Execute` sends `StartPending`, starts the worker goroutines, then sends
  `Running`. The auto updater runs in `serviceUpdateLoop`, a goroutine, so the
  control loop never blocks on a network check or a download. This keeps the
  service responsive to Stop and Interrogate at all times.
- The control loop reads only from the request channel `r`. On Stop or
  Shutdown it sends `StopPending` and returns. The `svc.Run` framework sends
  `Stopped` after `Execute` returns, so the code does not send it.
- On a new version, `handleUpdate` calls `os.Exit(1)`. The process ends without
  the `Stopped` state, so the Service Control Manager sees a failure and the
  recovery actions restart the service.
- `eventlog.InstallAsEventCreate` registers the log source at install time.
  `runService` opens the source with `eventlog.Open` and falls back to a debug
  logger if the open fails.

## Logging

The code logs with the standard library `log/slog`. `slog.Info` and `slog.Error`
carry structured attributes, for example
`slog.Error("failed to post status", "err", err)`.

The default logger uses `eventLogHandler`, a small `slog.Handler` in `helper.go`.
For each record it renders `message key=value ...` and writes it to the active
event log through `currentSink`: `elog` (the service event log) in the service
process, else `elogClient` (the bootstrap event log), else standard error. It
maps `slog.LevelError` to `Error`, `slog.LevelWarn` to `Warning`, and lower
levels to `Info`. The event log records the timestamp, so the handler does not
add one.

`logStep(name, err)` logs the result of a startup step: an error when it failed,
else `ok`.

The one output that is not a log is the `screenshot:<path>` line on standard
output; that is protocol output the caller reads. Per-keystroke trace prints in
`keyboard.go` were removed, because they would flood the event log.
