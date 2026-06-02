# chcsv-go

Go port of **chcsv** — an Oracle database to CSV export tool, originally written in Pro\*C by Batayan.

## Overview

`chcsvgo` connects to an Oracle database, executes a SQL query (from stdin or file), and outputs the results as CSV. It is a drop-in replacement for the original `chcsv` Pro\*C tool.

## Requirements

- Go 1.21+
- Oracle Instant Client (required by [godror](https://github.com/godror/godror))

## Build

```bash
go build -o chcsvgo .
```

### Static build (Linux)

```bash
CGO_ENABLED=0 go build -o chcsvgo .
```

> **Note:** godror uses CGO to link against Oracle Instant Client. For a fully static binary, ensure Oracle Instant Client libraries are available on the target system.

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
| `-f <n>` | Oracle array fetch size | 100 |
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

## Behavior notes

- **NULL values** are output as empty fields.
- **Numeric columns** (NUMBER, FLOAT, BINARY\_FLOAT, BINARY\_DOUBLE, INTEGER) are never enclosed, matching the original chcsv behavior.
- **Bind variables** use Oracle named syntax (`:varname`). Values are mapped positionally from command-line arguments.
- **Signals** (SIGHUP, SIGINT, SIGQUIT, SIGTERM) cancel the query and exit cleanly.

## License

See original chcsv for licensing terms.
