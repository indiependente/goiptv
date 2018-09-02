package filter

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// Filter filters out unwanted lines from the input reader
func Filter(r io.Reader) (bytes.Buffer, error) {
	var (
		b   bytes.Buffer
		err error
	)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if line := scanner.Text(); strings.HasPrefix(line, "#") || strings.HasPrefix(line, "http") {
			n, err := b.WriteString(line + "\n")
			if err != nil {
				return b, fmt.Errorf("%d bytes written. Error while writing line to buffer: %v", n, err)
			}
		}
	}
	if err = scanner.Err(); err != nil {
		return b, fmt.Errorf("Error while scanning: %v", err)
	}
	return b, nil
}
