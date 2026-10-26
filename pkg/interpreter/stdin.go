// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package interpreter

import (
	"bufio"
	"os"
	"strings"
	"sync"
)

var (
	stdinMu     sync.Mutex
	stdinReader *bufio.Reader
)

// ReadStdinLine reads a single line from standard input. Every input helper
// (input(), cli.prompt, cli.confirm, the '<<' stream) shares one reader, so
// consecutive reads do not lose data that an earlier read has buffered.
func ReadStdinLine() (string, bool) {
	stdinMu.Lock()
	defer stdinMu.Unlock()
	if stdinReader == nil {
		stdinReader = bufio.NewReader(os.Stdin)
	}
	line, err := stdinReader.ReadString('\n')
	if err != nil && line == "" {
		return "", false
	}
	return strings.TrimRight(line, "\r\n"), true
}
