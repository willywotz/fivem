# Context

## Auto updater — fully background

The auto updater runs without any visible effect on the user.

- `update()` (`update.go`) starts one background goroutine. It does the first
  check inside the goroutine, so program startup never waits for the network.
- `handleUpdate()` gives no console output. It writes only errors, through
  `failedf` (event log in the service, standard error otherwise).
- On a new version, the restart is silent. The new process starts with no
  standard handles and the `CREATE_NO_WINDOW` flag, so no console window shows.
- The Windows service path is not changed. It exits with `os.Exit(1)` and
  Service Recovery restarts it.
