package base

import (
	"bufio"
	"io"
	"strings"
)

// MaxScannerBuffer is the maximum size the scanner buffer can grow to (256KB).
const MaxScannerBuffer = 256 * 1024

// PendingToolCall tracks incremental assembly of a tool call across stream events.
type PendingToolCall struct {
	ID   string
	Name string
	Args strings.Builder
}

// SSEScanner wraps bufio.Scanner to parse Server-Sent Events (SSE) streams.
// It handles "data: " prefixed lines, skips empty lines, and detects "[DONE]".
type SSEScanner struct {
	scanner *bufio.Scanner
	done    bool
	err     error
}

// NewSSEScanner creates a new SSEScanner reading from the given reader.
// The scanner buffer is configured with a 4KB initial buffer that can grow to MaxScannerBuffer.
func NewSSEScanner(reader io.Reader) *SSEScanner {
	s := &SSEScanner{}
	s.scanner = bufio.NewScanner(reader)
	s.scanner.Buffer(make([]byte, 4096), MaxScannerBuffer)
	return s
}

// Next advances to the next SSE data event. It returns:
//   - eventType: currently always "" (reserved for future use)
//   - data: the parsed data payload (without the "data: " prefix), or nil for "[DONE]"
//   - err: any scanning error encountered
//
// When the stream sends "data: [DONE]", data will be nil with no error.
// Callers should stop iterating when data is nil and err is nil.
func (s *SSEScanner) Next() (eventType string, data []byte, err error) {
	for s.scanner.Scan() {
		line := s.scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			s.done = true
			return "", nil, nil
		}
		return "", []byte(payload), nil
	}
	if err := s.scanner.Err(); err != nil {
		s.err = err
		return "", nil, err
	}
	// Stream ended without [DONE]
	return "", nil, io.EOF
}
