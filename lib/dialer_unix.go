//go:build !windows

package lib

import (
	"errors"
	"syscall"
)

// Проверка ошибки отказа соединения для POSIX-систем
func isConnectionRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED)
}

// Проверка ошибки недоступности сети или хоста для POSIX-систем
func isNetworkUnreachable(err error) bool {
	return errors.Is(err, syscall.ENETUNREACH) ||
		errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ENETDOWN) ||
		errors.Is(err, syscall.EHOSTDOWN)
}
