package main

import (
	"bufio"
	"io"
)

type csvWriter struct {
	w         *bufio.Writer
	terminate byte
	enclosure byte
}

func newCSVWriter(w io.Writer, cfg *Config) *csvWriter {
	bufSize := cfg.OutRecords * 256
	if bufSize < 4096 {
		bufSize = 4096
	}
	return &csvWriter{
		w:         bufio.NewWriterSize(w, bufSize),
		terminate: cfg.Terminate,
		enclosure: cfg.Enclosure,
	}
}

func (c *csvWriter) writeHeader(names []string, isNum []bool) {
	for i, name := range names {
		if i > 0 {
			c.w.WriteByte(c.terminate)
		}
		if c.enclosure != 0 {
			c.w.WriteByte(c.enclosure)
		}
		c.w.WriteString(name)
		if c.enclosure != 0 {
			c.w.WriteByte(c.enclosure)
		}
	}
	c.w.WriteByte('\n')
}

// writeRow writes one CSV row. Enclosure is skipped for numeric columns,
// matching the original chcsv behavior (numflg check in OutPut.pc).
func (c *csvWriter) writeRow(vals []string, isNum []bool) {
	for i, v := range vals {
		if i > 0 {
			c.w.WriteByte(c.terminate)
		}
		if c.enclosure != 0 && !isNum[i] {
			c.w.WriteByte(c.enclosure)
		}
		c.w.WriteString(v)
		if c.enclosure != 0 && !isNum[i] {
			c.w.WriteByte(c.enclosure)
		}
	}
	c.w.WriteByte('\n')
}

func (c *csvWriter) flush() error {
	return c.w.Flush()
}
