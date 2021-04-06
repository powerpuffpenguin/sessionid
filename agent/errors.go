package agent

import "errors"

// Error constants
var (
	ErrAgentClosed = errors.New("agent already closed")
)
