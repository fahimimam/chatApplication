package tcp

import (
	"errors"
	"strings"
)

// ParseCommand splits a single client line into a verb and arguments. An
// empty line or one starting with '#' is returned as a comment so the
// server can ignore it.
func ParseCommand(line string) (verb string, args []string, comment bool, err error) {
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return "", nil, true, nil
	}
	if strings.HasPrefix(line, "#") {
		return "", nil, true, nil
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", nil, true, nil
	}
	return fields[0], fields[1:], false, nil
}

// ErrMalformed is returned by the transport for protocol violations.
var ErrMalformed = errors.New("malformed command")