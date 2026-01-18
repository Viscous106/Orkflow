package memory

import "errors"

// Common errors for the memory package
var (
	// ErrChannelClosed is returned when trying to send on a closed MessageChannel
	ErrChannelClosed = errors.New("message channel is closed")
)
