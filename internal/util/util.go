package util

import (
	"context"
	"errors"
	"io"
	"net"
	"syscall"
)

func IsExpectedCopyError(err error) bool {
	if err == nil {
		return true
	}

	if errors.Is(err, io.EOF) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, context.Canceled) {
		return true
	}

	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}

	return false
}
