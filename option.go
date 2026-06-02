package main

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all runtime configuration parsed from command-line arguments.
type Config struct {
	ConnStr    string   // user/password[@connectString]
	OutFile    string   // -o output file path
	InFile     string   // -i SQL input file path
	Enclosure  byte     // -e enclosure char (0 = none)
	Terminate  byte     // -t column terminator (default ',')
	LongSize   int      // -l LONG type max bytes
	FetchArray int      // -f Oracle array fetch size
	OutRecords int      // -b output buffer size in records
	Vertical   bool     // -v one column per line
	Header     bool     // -h output column name header
	Append     bool     // -a append to output file
	NoDataExit bool     // -n exit non-zero when no rows returned
	BindValues []string // positional bind variable values
}

func parseOptions(args []string) *Config {
	cfg := &Config{
		Terminate:  ',',
		LongSize:   1000,
		FetchArray: 10000,
		OutRecords: 100,
	}

	if len(args) < 1 {
		optionErr()
	}

	cfg.ConnStr = args[0]
	args = args[1:]

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if len(arg) == 0 || arg[0] != '-' {
			cfg.BindValues = append(cfg.BindValues, arg)
			continue
		}
		if len(arg) < 2 {
			optionErr()
		}

		nextArg := func() string {
			if len(arg) > 2 {
				return arg[2:]
			}
			i++
			if i >= len(args) {
				optionErr()
			}
			return args[i]
		}

		switch arg[1] {
		case 'h':
			cfg.Header = true
		case 'v':
			cfg.Vertical = true
		case 'n':
			cfg.NoDataExit = true
		case 'a':
			cfg.Append = true
			cfg.OutFile = nextArg()
		case 'o':
			cfg.OutFile = nextArg()
		case 'i':
			cfg.InFile = nextArg()
		case 't':
			v := nextArg()
			if len(v) == 0 {
				optionErr()
			}
			cfg.Terminate = v[0]
		case 'e':
			v := nextArg()
			if len(v) == 0 {
				optionErr()
			}
			cfg.Enclosure = v[0]
		case 'l':
			n, err := strconv.Atoi(nextArg())
			if err != nil || n <= 0 {
				optionErr()
			}
			cfg.LongSize = n
		case 'f':
			n, err := strconv.Atoi(nextArg())
			if err != nil || n <= 0 {
				optionErr()
			}
			cfg.FetchArray = n
		case 'b':
			n, err := strconv.Atoi(nextArg())
			if err != nil || n <= 0 {
				optionErr()
			}
			cfg.OutRecords = n
		default:
			optionErr()
		}
	}

	if cfg.Vertical {
		cfg.Terminate = '\n'
	}

	return cfg
}

func optionErr() {
	fmt.Fprintf(os.Stderr, "chcsvGo - Oracle to CSV converter (Go port of chcsv v2.0)\n\n")
	fmt.Fprintf(os.Stderr, "Usage: chcsvgo userid/password[@host]\n")
	fmt.Fprintf(os.Stderr, "\t[-o Output Filename]\n")
	fmt.Fprintf(os.Stderr, "\t[-a Appended Output Filename]\n")
	fmt.Fprintf(os.Stderr, "\t[-i Input Filename (SQL)]\n")
	fmt.Fprintf(os.Stderr, "\t[-e Enclosure char]\n")
	fmt.Fprintf(os.Stderr, "\t[-t Terminator char]\n")
	fmt.Fprintf(os.Stderr, "\t[-l Length of LONG type]\n")
	fmt.Fprintf(os.Stderr, "\t[-f Array fetch size]\n")
	fmt.Fprintf(os.Stderr, "\t[-b Output buffer size (records)]\n")
	fmt.Fprintf(os.Stderr, "\t[-v] [-h] [-n]\n")
	fmt.Fprintf(os.Stderr, "\t[bind_value] [bind_value] ...\n")
	os.Exit(1)
}
