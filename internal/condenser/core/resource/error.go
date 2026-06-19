package resource

import "fmt"

type StatusError struct {
	Status  int
	Message string
}

func (e StatusError) Error() string {
	return e.Message
}

func statusError(status int, format string, args ...any) error {
	return StatusError{
		Status:  status,
		Message: fmt.Sprintf(format, args...),
	}
}

func statusMessage(status int, message string) error {
	return StatusError{
		Status:  status,
		Message: message,
	}
}

func ErrorStatus(err error, fallback int) int {
	if err == nil {
		return fallback
	}
	if statusErr, ok := err.(StatusError); ok {
		return statusErr.Status
	}
	return fallback
}
