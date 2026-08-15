package main

import "fmt"

type usageError struct {
	message string
}

func (e usageError) Error() string { return e.message }

func usagef(format string, args ...any) error {
	return usageError{message: fmt.Sprintf(format, args...)}
}

func isUsageError(err error) bool {
	_, ok := err.(usageError)
	return ok
}
