package lib

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// Проверка открытого порта через локальный TCP-листенер
func TestProbeTCPOpen(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer ln.Close()

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("failed to split host port: %v", err)
	}
	portNum, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("invalid port number %q: %v", portStr, err)
	}

	target := resolvedTarget{
		Host: "127.0.0.1",
		IP:   net.ParseIP("127.0.0.1"),
	}

	res := probeTCP(context.Background(), target, uint16(portNum), dialConfig{})
	if res.State != StateOpen {
		t.Fatalf("got state %v, want %v (err: %v)", res.State, StateOpen, res.Err)
	}
	if res.Err != nil {
		t.Fatalf("expected nil error for open port, got %v", res.Err)
	}
	if res.Host != target.Host || !res.IP.Equal(target.IP) || res.Port != uint16(portNum) {
		t.Fatalf("unexpected result metadata: %+v", res)
	}
	if res.Duration <= 0 {
		t.Fatalf("expected positive duration, got %v", res.Duration)
	}
}

// Проверка открытого порта по IPv6 loopback при наличии поддержки в системе
func TestProbeTCPOpenIPv6(t *testing.T) {
	ln, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("skipping IPv6 listener test: %v", err)
	}
	defer ln.Close()

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("failed to split host port: %v", err)
	}
	portNum, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("invalid port number %q: %v", portStr, err)
	}

	target := resolvedTarget{
		Host: "::1",
		IP:   net.ParseIP("::1"),
	}
	cfg := dialConfig{
		Dialer: &net.Dialer{Timeout: 200 * time.Millisecond},
	}

	res := probeTCP(context.Background(), target, uint16(portNum), cfg)
	if res.State != StateOpen {
		t.Fatalf("got state %v, want %v (err: %v)", res.State, StateOpen, res.Err)
	}
	if res.Err != nil {
		t.Fatalf("expected nil error for open IPv6 port, got %v", res.Err)
	}
}

// Проверка закрытого порта на локальном интерфейсе
func TestProbeTCPClosed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve port: %v", err)
	}
	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("failed to split host port: %v", err)
	}
	_ = ln.Close()

	portNum, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("invalid port number %q: %v", portStr, err)
	}

	target := resolvedTarget{
		Host: "127.0.0.1",
		IP:   net.ParseIP("127.0.0.1"),
	}
	cfg := dialConfig{
		Dialer: &net.Dialer{Timeout: 200 * time.Millisecond},
	}

	res := probeTCP(context.Background(), target, uint16(portNum), cfg)
	if res.State != StateClosed {
		t.Fatalf("got state %v, want %v (err: %v)", res.State, StateClosed, res.Err)
	}
	if res.Err == nil {
		t.Fatal("expected non-nil error for closed port")
	}
}

// Проверка уже отменённого контекста при подключении
func TestProbeTCPCanceledBeforeDial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	target := resolvedTarget{
		Host: "127.0.0.1",
		IP:   net.ParseIP("127.0.0.1"),
	}
	cfg := dialConfig{
		Dialer: &net.Dialer{Timeout: 200 * time.Millisecond},
	}

	res := probeTCP(ctx, target, 80, cfg)
	if res.State != StateCanceled {
		t.Fatalf("got state %v, want %v (err: %v)", res.State, StateCanceled, res.Err)
	}
	if !errors.Is(res.Err, context.Canceled) {
		t.Fatalf("got error %v, want context.Canceled", res.Err)
	}
}

// Проверка отмены контекста во время активного ожидания подключения
func TestProbeTCPCanceledDuringDial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(15 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	target := resolvedTarget{
		Host: "192.0.2.1",
		IP:   net.ParseIP("192.0.2.1"),
	}
	cfg := dialConfig{
		Dialer: &net.Dialer{Timeout: 2 * time.Second},
	}

	res := probeTCP(ctx, target, 81, cfg)
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Fatalf("dial did not abort promptly on context cancel: elapsed %v", elapsed)
	}
	if res.State != StateCanceled && res.State != StateUnreachable {
		t.Fatalf("got state %v, want %v or %v (err: %v)", res.State, StateCanceled, StateUnreachable, res.Err)
	}
	if res.State == StateCanceled && !errors.Is(res.Err, context.Canceled) {
		t.Fatalf("got error %v, want context.Canceled", res.Err)
	}
}

// Проверка таймаута или недоступности на немаршрутизируемом адресе
func TestProbeTCPTimeoutOrUnreachable(t *testing.T) {
	target := resolvedTarget{
		Host: "192.0.2.1",
		IP:   net.ParseIP("192.0.2.1"),
	}
	cfg := dialConfig{
		Dialer: &net.Dialer{Timeout: 30 * time.Millisecond},
	}

	res := probeTCP(context.Background(), target, 81, cfg)
	if res.State != StateTimeout && res.State != StateUnreachable {
		t.Fatalf("got state %v, want %v or %v (err: %v)", res.State, StateTimeout, StateUnreachable, res.Err)
	}
	if res.Err == nil {
		t.Fatal("expected non-nil error")
	}
	if res.Duration <= 0 {
		t.Fatalf("expected positive duration, got %v", res.Duration)
	}
}

// Проверка классификации сетевых ошибок и ошибок контекста
func TestClassifyError(t *testing.T) {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name string
		ctx  context.Context
		err  error
		want State
	}{
		{
			name: "canceled context",
			ctx:  canceledCtx,
			err:  context.Canceled,
			want: StateCanceled,
		},
		{
			name: "wrapped context canceled",
			ctx:  context.Background(),
			err:  &net.OpError{Op: "dial", Net: "tcp", Err: context.Canceled},
			want: StateCanceled,
		},
		{
			name: "deadline exceeded",
			ctx:  context.Background(),
			err:  context.DeadlineExceeded,
			want: StateTimeout,
		},
		{
			name: "connection refused via syscall error",
			ctx:  context.Background(),
			err:  os.NewSyscallError("connect", syscall.ECONNREFUSED),
			want: StateClosed,
		},
		{
			name: "network unreachable",
			ctx:  context.Background(),
			err:  os.NewSyscallError("connect", syscall.ENETUNREACH),
			want: StateUnreachable,
		},
		{
			name: "host unreachable",
			ctx:  context.Background(),
			err:  os.NewSyscallError("connect", syscall.EHOSTUNREACH),
			want: StateUnreachable,
		},
		{
			name: "generic error",
			ctx:  context.Background(),
			err:  errors.New("unexpected network failure"),
			want: StateError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyError(tc.ctx, tc.err)
			if got != tc.want {
				t.Fatalf("got state %v, want %v", got, tc.want)
			}
		})
	}
}
