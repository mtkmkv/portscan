package lib

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"time"
)

// Конфигурация TCP-подключения
type dialConfig struct {
	Dialer *net.Dialer
}

// Выполнение TCP Connect и формирование результата проверки порта
func probeTCP(ctx context.Context, target resolvedTarget, port uint16, cfg dialConfig) Result {
	start := time.Now()

	if err := ctx.Err(); err != nil {
		return Result{
			Host:     target.Host,
			IP:       target.IP,
			Port:     port,
			State:    classifyError(ctx, err),
			Duration: time.Since(start),
			Err:      err,
		}
	}

	dialer := cfg.Dialer
	if dialer == nil {
		dialer = &net.Dialer{}
	}

	conn, err := dialer.DialContext(ctx, "tcp", formatTargetAddr(target.IP, port))
	duration := time.Since(start)
	if err != nil {
		return Result{
			Host:     target.Host,
			IP:       target.IP,
			Port:     port,
			State:    classifyError(ctx, err),
			Duration: duration,
			Err:      err,
		}
	}

	_ = conn.Close()

	return Result{
		Host:     target.Host,
		IP:       target.IP,
		Port:     port,
		State:    StateOpen,
		Duration: duration,
		Err:      nil,
	}
}

// Формирование адреса хост:порт для IPv4 и IPv6
func formatTargetAddr(ip net.IP, port uint16) string {
	portStr := strconv.Itoa(int(port))
	if addr, ok := netip.AddrFromSlice(ip); ok {
		addr = addr.Unmap()
		return net.JoinHostPort(addr.String(), portStr)
	}
	return net.JoinHostPort(ip.String(), portStr)
}

// Классификация ошибки подключения или контекста в состояние результата сканирования
func classifyError(ctx context.Context, err error) State {
	if ctx != nil && errors.Is(ctx.Err(), context.Canceled) {
		return StateCanceled
	}
	if errors.Is(err, context.Canceled) {
		return StateCanceled
	}
	if ctx != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return StateTimeout
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return StateTimeout
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return StateTimeout
	}

	if isConnectionRefused(err) {
		return StateClosed
	}

	if isNetworkUnreachable(err) {
		return StateUnreachable
	}

	return StateError
}
