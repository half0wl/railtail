package logger

import (
	"log/slog"
	"strings"
)

func ErrAttr(err error) slog.Attr {
	return slog.String("err", strings.TrimSpace(err.Error()))
}

func ErrorsAttr(errors ...error) slog.Attr {
	stringErrors := []string{}

	for _, err := range errors {
		stringErrors = append(stringErrors, strings.TrimSpace(err.Error()))
	}

	return slog.Any("errors", stringErrors)
}
