# chcsv-go

Go port of **chcsv** — an Oracle database to CSV export tool, originally written in Pro\*C by Batayan.

## Overview

`chcsvgo` connects to an Oracle database, executes a SQL query (from stdin or file), and outputs the results as CSV. It is a drop-in replacement for the original `chcsv` Pro\*C tool.

## Requirements

- Go 1.21+
- Oracle Instant Client (required by [godror](https://github.com/godror/godror))

## Build

> **Note:** godror uses **CGO** and dynamically loads Oracle Instant Client **at runtime**.
> Because of this, build **natively on the target OS/arch** (CGO cross-compilation is not practical here),
> and make sure the Instant Client libraries are reachable via the loader path when running the binary.
> `CGO_ENABLED=0` (a fully static build) is **not** supported by godror.

### macOS (Apple Silicon / arm64)

```bash
# 1. Install Oracle Instant Client (Basic) for macOS arm64
#    Download: https://www.oracle.com/database/technologies/instant-client/macos-arm64-downloads.html
#    Unzip it, then point the dynamic loader at it:
export DYLD_LIBRARY_PATH="$HOME/lib/instantclient_23_3:$DYLD_LIBRARY_PATH"

# 2. Build natively (arm64)
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o chcsvgo .
```

### Linux (x86_64)

```bash
# 1. Install Oracle Instant Client (Basic + Devel)
#    Oracle Linux / RHEL example:
sudo dnf install -y oracle-instantclient-basic oracle-instantclient-devel
#    Or unzip the Basic package and export its path:
export LD_LIBRARY_PATH=/opt/oracle/instantclient_19_28:$LD_LIBRARY_PATH

# 2. Build natively (amd64)
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o chcsvgo .
```

## Docker

A multi-stage `Dockerfile` is provided that bundles the Oracle Instant Client,
so you can build and run without installing Go or the Instant Client on the host.

### Build the image

```bash
# Classic builder: TARGETARCH must be passed explicitly
docker build --build-arg TARGETARCH=amd64 -t chcsvgo:latest .

# BuildKit / buildx: TARGETARCH is auto-detected
docker buildx build -t chcsvgo:latest .
```

### Run

```bash
# Pipe SQL via stdin (-i keeps stdin open). Connect to any reachable DB:
echo "SELECT * FROM employees" | \
  docker run --rm -i chcsvgo:latest 'user/pass@host:1521/service' -h

# Connect to an Oracle container on the same Docker network:
echo "SELECT * FROM employees" | \
  docker run --rm -i --network oracle_default \
    chcsvgo:latest 'system/pass@oracle-db:1521/ORCLPDB1' -h

# Read a SQL file and write CSV by mounting the working directory:
docker run --rm -i -v "$PWD:/work" -w /work \
  chcsvgo:latest 'user/pass@host:1521/service' -i query.sql -o out.csv -h
```

## Usage

```
chcsvgo userid/password[@connectString] [options] [bind_value ...]
```

### Connection string formats

```bash
# Local / TNS alias
chcsvgo user/password@ORCL

# Easy Connect
chcsvgo user/password@host:1521/service_name

# No connect string (uses TWO_TASK / ORACLE_SID env)
chcsvgo user/password
```

### Options

| Option | Description | Default |
|--------|-------------|---------|
| `-o <file>` | Output file path | stdout |
| `-a <file>` | Append to output file | stdout |
| `-i <file>` | SQL input file | stdin |
| `-e <char>` | Enclosure character (e.g. `"`) | none |
| `-t <char>` | Column terminator character | `,` |
| `-l <n>` | Max bytes for LONG type | 1000 |
| `-f <n>` | Oracle array fetch size | 10000 |
| `-b <n>` | Output buffer size (records) | 100 |
| `-h` | Output column name header row | off |
| `-v` | Vertical mode (one column per line) | off |
| `-n` | Exit with code 1 if no rows returned | off |

Positional arguments after options are passed as bind variable values in order of first appearance in the SQL (`:var1`, `:var2`, ...).

## Examples

```bash
# Basic query from stdin
echo "SELECT * FROM employees" | chcsvgo user/pass@localhost:1521/XEPDB1

# With header row and quoted strings
echo "SELECT id, name FROM employees" | chcsvgo user/pass@mydb -h -e '"'

# Read SQL from file, write CSV to file
chcsvgo user/pass@mydb -i query.sql -o output.csv -h

# With bind variables
echo "SELECT * FROM orders WHERE status = :1" | chcsvgo user/pass@mydb active

# Tab-separated output
echo "SELECT * FROM employees" | chcsvgo user/pass@mydb -t $'\t'

# Exit non-zero if no data (useful in shell scripts)
echo "SELECT * FROM alerts WHERE level = :1" | chcsvgo user/pass@mydb -n CRITICAL
echo $?  # 1 if no rows, 0 if rows found
```

## Performance tuning

### Array fetch size (`-f`)

`-f` controls the **Oracle array fetch size** — how many rows the client pulls from
the database per network round trip. It is the single biggest lever for export speed
on large result sets.

- A larger value means **fewer round trips**, which dramatically reduces wall-clock
  time when exporting many rows, especially over higher-latency networks (remote DB,
  VPN, Tailscale, etc.).
- A larger value also means **more client-side memory** per fetch (roughly
  `fetch_size × row_width`), so very wide rows (many/large columns, LONG, big VARCHARs)
  may warrant a smaller value.

The default is **10000**, which is a good balance for typical narrow-to-medium rows.
Tune it for your workload:

```bash
# High latency / narrow rows: push the fetch size up for max throughput
echo "SELECT id, name FROM big_table" | chcsvgo user/pass@mydb -f 50000 -o out.csv

# Very wide rows / memory constrained: lower it to cap per-fetch memory
echo "SELECT * FROM wide_table" | chcsvgo user/pass@mydb -f 1000 -o out.csv
```

> **Rule of thumb:** start with the default, then raise `-f` if exports are slow and
> the rows are narrow; lower it if the process uses too much memory on wide rows.
> The related `-b` (output buffer size in records) controls how often output is
> flushed and has a much smaller impact than `-f`.

## Behavior notes

- **NULL values** are output as empty fields.
- **Numeric columns** (NUMBER, FLOAT, BINARY\_FLOAT, BINARY\_DOUBLE, INTEGER) are never enclosed, matching the original chcsv behavior.
- **Bind variables** support both named (`:varname`) and positional/numeric (`:1`, `:2`) syntax. Values are mapped positionally from command-line arguments, in order of first appearance.
- **Colons inside string literals, quoted identifiers, and comments are ignored**, so date masks such as `TO_CHAR(d, 'HH24:MI:SS')` do not need any bind values.
- **Signals** (SIGHUP, SIGINT, SIGQUIT, SIGTERM) cancel the query and exit cleanly.

## License

See original chcsv for licensing terms.
