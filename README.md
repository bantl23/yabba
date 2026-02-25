# yabba

Yet Another Blah Bandwidth Analyzer — a lightweight TCP bandwidth testing tool similar to iperf.

## Installation

```bash
go install github.com/bantl23/yabba@latest
```

Or build from source:

```bash
git clone https://github.com/bantl23/yabba.git
cd yabba
task build
```

## Usage

Start a listener on the server side:

```bash
yabba listen
```

Run the client to measure throughput:

```bash
yabba connect
```

### Listen options

| Flag | Default | Description |
|------|---------|-------------|
| `-a`, `--addr` | `:5201` | Bind address |
| `-s`, `--size` | `131072` | Read buffer size (bytes) |

### Connect options

| Flag | Default | Description |
|------|---------|-------------|
| `-a`, `--addrs` | `localhost:5201` | Server address(es), comma-separated |
| `-c`, `--connections` | `1` | Parallel connections per address |
| `-d`, `--duration` | `10s` | Test duration |
| `-s`, `--size` | `131072` | Write buffer size (bytes) |

### Examples

Run a 30-second test with 4 parallel connections:

```bash
# server
yabba listen -a :5201

# client
yabba connect -a 192.168.1.10:5201 -c 4 -d 30s
```

Test against multiple servers simultaneously:

```bash
yabba connect -a 192.168.1.10:5201,192.168.1.11:5201 -c 2 -d 10s
```

Print the version:

```bash
yabba version
```

## Output

Results are emitted as structured JSON logs. Each connection reports its individual rate, and a final aggregate is printed per address:

```json
{"level":"info","msg":"all connected"}
{"address":"127.0.0.1:54321","level":"info","msg":"rate","mbps":941.2}
{"level":"info","mbps":941.2,"msg":"rate average","remote":"127.0.0.1:5201"}
```

## Development

```bash
task build   # build binary
task test    # run tests with coverage
task clean   # remove binary
```

Requires [Task](https://taskfile.dev) (`go install github.com/go-task/task/v3/cmd/task@latest`).
