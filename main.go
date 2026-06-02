// chcsvGo - Oracle to CSV converter, Go port of chcsv (Pro*C) by Batayan.
//
// Usage: chcsvgo user/password[@host] [options] [bind_value ...]
//
// See optionErr() in option.go for the full option list.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg := parseOptions(os.Args[1:])

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		fmt.Fprintf(os.Stderr, "chcsvgo: caught signal %v, aborting\n", sig)
		cancel()
	}()

	sqlStr, err := readSQL(cfg)
	if err != nil {
		fatalf("read SQL: %v", err)
	}

	out, closeOut, err := openOutput(cfg)
	if err != nil {
		fatalf("open output: %v", err)
	}
	defer closeOut()

	db, err := connectOracle(cfg.ConnStr)
	if err != nil {
		fatalf("oracle: %v", err)
	}
	defer func() {
		db.ExecContext(context.Background(), "COMMIT")
		db.Close()
	}()

	w := newCSVWriter(out, cfg)

	noData, err := execQuery(ctx, db, cfg, sqlStr, w)
	if err != nil {
		fatalf("query: %v", err)
	}

	if err := w.flush(); err != nil {
		fatalf("flush: %v", err)
	}

	if cfg.NoDataExit && noData {
		os.Exit(1)
	}
}

func readSQL(cfg *Config) (string, error) {
	var r io.Reader
	if cfg.InFile != "" {
		f, err := os.Open(cfg.InFile)
		if err != nil {
			return "", err
		}
		defer f.Close()
		r = f
	} else {
		r = os.Stdin
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func openOutput(cfg *Config) (io.Writer, func(), error) {
	if cfg.OutFile == "" {
		return os.Stdout, func() {}, nil
	}
	flags := os.O_WRONLY | os.O_CREATE
	if cfg.Append {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(cfg.OutFile, flags, 0666)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "chcsvgo: "+format+"\n", args...)
	os.Exit(1)
}
