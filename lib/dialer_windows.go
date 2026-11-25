//go:build windows

package lib

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows"
)

// Проверка ошибки отказа соединения для Windows
func isConnectionRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, windows.WSAECONNREFUSED)
}

// Проверка ошибки недоступности сети или хоста для Windows
func isNetworkUnreachable(err error) bool {
	return errors.Is(err, syscall.ENETUNREACH) ||
		errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ENETDOWN) ||
		errors.Is(err, syscall.EHOSTDOWN) ||
		errors.Is(err, windows.WSAENETUNREACH) ||
		errors.Is(err, windows.WSAEHOSTUNREACH) ||
		errors.Is(err, windows.WSAENETDOWN)
}
