package lib

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"
)

// Проверка валидации конструктора и опций сканера
func TestScannerNewAndOptions(t *testing.T) {
	if _, err := New(WithConcurrency(0)); !errors.Is(err, ErrInvalidConcurrency) {
		t.Fatalf("got error %v, want %v", err, ErrInvalidConcurrency)
	}
	if _, err := New(WithConcurrency(-5)); !errors.Is(err, ErrInvalidConcurrency) {
		t.Fatalf("got error %v, want %v", err, ErrInvalidConcurrency)
	}
	if _, err := New(WithConnectTimeout(0)); !errors.Is(err, ErrInvalidTimeout) {
		t.Fatalf("got error %v, want %v", err, ErrInvalidTimeout)
	}
	if _, err := New(WithConnectTimeout(-10 * time.Millisecond)); !errors.Is(err, ErrInvalidTimeout) {
		t.Fatalf("got error %v, want %v", err, ErrInvalidTimeout)
	}

	s, err := New(nil, WithConcurrency(16), WithConnectTimeout(150*time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error creating scanner: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil scanner")
	}
}

// Проверка ошибок валидации при вызове Scan
func TestScannerScanValidationErrors(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("unexpected error creating scanner: %v", err)
	}

	var nilScanner *Scanner
	if _, err := nilScanner.Scan(context.Background(), []string{"127.0.0.1"}, Ports(80)); !errors.Is(err, ErrNilScanner) {
		t.Fatalf("got error %v, want %v", err, ErrNilScanner)
	}

	//nolint:staticcheck // intentionally testing nil context handling
	if _, err := s.Scan(nil, []string{"127.0.0.1"}, Ports(80)); !errors.Is(err, ErrNilContext) {
		t.Fatalf("got error %v, want %v", err, ErrNilContext)
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Scan(canceledCtx, []string{"127.0.0.1"}, Ports(80)); !errors.Is(err, context.Canceled) {
		t.Fatalf("got error %v, want %v", err, context.Canceled)
	}

	if _, err := s.Scan(context.Background(), []string{"127.0.0.1"}); !errors.Is(err, ErrEmptyPorts) {
		t.Fatalf("got error %v, want %v", err, ErrEmptyPorts)
	}

	if _, err := s.Scan(context.Background(), []string{"127.0.0.1"}, nil); !errors.Is(err, ErrNilPortSpec) {
		t.Fatalf("got error %v, want %v", err, ErrNilPortSpec)
	}

	if _, err := s.Scan(context.Background(), nil, Ports(80)); !errors.Is(err, ErrEmptyHosts) {
		t.Fatalf("got error %v, want %v", err, ErrEmptyHosts)
	}
}

// Сквозное сканирование открытого и закрытого портов на loopback
func TestScannerScanOpenAndClosedPorts(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer ln.Close()

	_, openPortStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("failed to split host port: %v", err)
	}
	openPortNum, err := strconv.Atoi(openPortStr)
	if err != nil {
		t.Fatalf("invalid open port %q: %v", openPortStr, err)
	}
	openPort := uint16(openPortNum)

	closedLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve closed port: %v", err)
	}
	_, closedPortStr, err := net.SplitHostPort(closedLn.Addr().String())
	if err != nil {
		t.Fatalf("failed to split closed host port: %v", err)
	}
	_ = closedLn.Close()

	closedPortNum, err := strconv.Atoi(closedPortStr)
	if err != nil {
		t.Fatalf("invalid closed port %q: %v", closedPortStr, err)
	}
	closedPort := uint16(closedPortNum)

	s, err := New(WithConcurrency(8), WithConnectTimeout(200*time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error creating scanner: %v", err)
	}

	resultsCh, err := s.Scan(context.Background(), []string{"127.0.0.1"}, Ports(openPort, closedPort))
	if err != nil {
		t.Fatalf("unexpected Scan error: %v", err)
	}

	statesByPort := make(map[uint16]State)
	for res := range resultsCh {
		statesByPort[res.Port] = res.State
	}

	if statesByPort[openPort] != StateOpen {
		t.Fatalf("port %d got state %v, want %v", openPort, statesByPort[openPort], StateOpen)
	}
	if statesByPort[closedPort] != StateClosed {
		t.Fatalf("port %d got state %v, want %v", closedPort, statesByPort[closedPort], StateClosed)
	}
}

/// Сквозной сценарий с Combine, несколькими целями и неразрешимым хостом
func TestScannerCombinedSpecAndMixedTargets(t *testing.T) {
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
		t.Fatalf("invalid port %q: %v", portStr, err)
	}
	openPort := uint16(portNum)

	s, err := New(WithConcurrency(8), WithConnectTimeout(200*time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error creating scanner: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	spec := Combine(Ports(openPort), Range(openPort+1, openPort+2))
	ch, err := s.Scan(ctx, []string{"127.0.0.1", "nonexistent.invalid.local"}, spec)
	if err != nil {
		t.Fatalf("unexpected Scan error: %v", err)
	}

	var results []Result
	for res := range ch {
		results = append(results, res)
	}

	// 3 порта × 2 цели (127.0.0.1 + неразрешимый хост с ошибкой резолвинга) = 6 результатов
	if len(results) != 6 {
		t.Fatalf("got %d results (%+v), want 6", len(results), results)
	}

	foundOpen := false
	foundResolveError := false
	for _, res := range results {
		if res.Host == "127.0.0.1" && res.Port == openPort && res.State == StateOpen {
			foundOpen = true
		}
		if res.Host == "nonexistent.invalid.local" && res.State == StateError && errors.Is(res.Err, ErrResolveFailure) {
			foundResolveError = true
		}
	}
	if !foundOpen {
		t.Fatalf("expected open port %d on 127.0.0.1 in results: %+v", openPort, results)
	}
	if !foundResolveError {
		t.Fatalf("expected resolve failure result for unresolvable host in results: %+v", results)
	}
}

// Отмена контекста во время сканирования при медленном читателе
func TestScannerCancelWithSlowReader(t *testing.T) {
	s, err := New(WithConcurrency(4), WithConnectTimeout(150*time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error creating scanner: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := s.Scan(ctx, []string{"127.0.0.1"}, Range(30000, 30030))
	if err != nil {
		t.Fatalf("unexpected Scan error: %v", err)
	}

	received := 0
	for res := range ch {
		_ = res
		received++
		if received == 2 {
			cancel()
		}
		time.Sleep(5 * time.Millisecond)
	}

	if received == 0 {
		t.Fatal("expected at least one result before cancellation")
	}
}

// Повторное и конкурентное использование одного экземпляра Scanner
func TestScannerConcurrentReuse(t *testing.T) {
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
		t.Fatalf("invalid port %q: %v", portStr, err)
	}
	port := uint16(portNum)

	s, err := New(WithConcurrency(10), WithConnectTimeout(200*time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error creating scanner: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(4)
	for range 4 {
		go func() {
			defer wg.Done()
			ch, scanErr := s.Scan(context.Background(), []string{"127.0.0.1"}, Ports(port))
			if scanErr != nil {
				t.Errorf("unexpected Scan error: %v", scanErr)
				return
			}
			var foundOpen bool
			for res := range ch {
				if res.Port == port && res.State == StateOpen {
					foundOpen = true
				}
			}
			if !foundOpen {
				t.Errorf("expected port %d to be reported open", port)
			}
		}()
	}
	wg.Wait()
}
