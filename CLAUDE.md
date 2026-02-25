# yabba

Yet Another Blah Bandwidth Analyzer — a TCP bandwidth testing tool similar to iperf.

## Commands

```bash
# Build (injects version/hash via ldflags)
task build

# Test with coverage
task test          # go test -v -cover ./...
go test -cover ./...

# Tidy modules
task mod

# Clean
task clean
```

## Project Structure

```
main.go          # entry point: sets up logrus JSON formatter, calls cmd.Execute()
cmd/
  root.go        # cobra root command ("yabba")
  connect.go     # "connect" subcommand → run.Client
  listen.go      # "listen" subcommand → run.Server
  version.go     # "version" subcommand; Version/Hash injected by ldflags at build time
run/
  client.go      # Client.Run(), clientRunTcp(), printTotals()
  server.go      # Server.Run(), serverRunTcp()
  stats.go       # Stats struct
  run_test.go    # full test suite (package run)
cmd/
  version_test.go
```

## Architecture

- **Server** (`listen`): binds TCP, accepts connections, dispatches each to `serverRunTcp` goroutine which reads until EOF and logs mbps.
- **Client** (`connect`): dials `Connections` goroutines per address, synchronises them via channels (`connectChan` → `beginChan` → `endChan`), writes for `Duration`, collects stats via `statsChan`, then calls `printTotals`.
- **Default port**: 5201 (same as iperf3).

## Key Conventions

- All logging uses **logrus** with JSON formatter (set in `main.go` `init()`). Never use `fmt.Println` for errors — use `log.WithError(err).Error(...)`.
- Cobra commands use `RunE` (not `Run`) so errors propagate to `main`.
- `serverRunTcp` allocates its own buffer (`make([]byte, size)`) — do not pass a shared buffer.
- `clientRunTcp` always signals `connectChan` on every return path and defers its `statsChan` send to prevent deadlocks in `Client.Run`.
- Division by zero guard: check `totalElapsed.Seconds() > 0` before computing mbps.

## Coverage

| Package | Coverage |
|---------|----------|
| `run`   | 92.6%    |
| `cmd`   | 72.2%    |

## Dependencies

- `github.com/spf13/cobra` v1.3.0 — CLI framework
- `github.com/sirupsen/logrus` v1.8.1 — structured JSON logging
