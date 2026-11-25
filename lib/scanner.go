package lib

import (
	"context"
	"errors"
	"net"
	"time"
)

const (
	defaultConcurrency    = 100
	defaultConnectTimeout = 500 * time.Millisecond
)

var (
	ErrNilScanner         = errors.New("nil scanner")
	ErrInvalidConcurrency = errors.New("invalid concurrency")
	ErrInvalidTimeout     = errors.New("invalid connect timeout")
	ErrNilContext         = errors.New("nil context")
	ErrNilPortSpec        = errors.New("nil port specification")
)

// Конфигуратор параметров сканера
type Option func(*scannerConfig) error

// Сканер доступности TCP-портов
type Scanner struct {
	concurrency int
	timeout     time.Duration
	resolver    *net.Resolver
}

type scannerConfig struct {
	concurrency int
	timeout     time.Duration
	resolver    *net.Resolver
}

// Ограничение числа одновременных проверок
func WithConcurrency(limit int) Option {
	return func(cfg *scannerConfig) error {
		if limit <= 0 {
			return ErrInvalidConcurrency
		}
		cfg.concurrency = limit
		return nil
	}
}

// Таймаут установки TCP-соединения
func WithConnectTimeout(timeout time.Duration) Option {
	return func(cfg *scannerConfig) error {
		if timeout <= 0 {
			return ErrInvalidTimeout
		}
		cfg.timeout = timeout
		return nil
	}
}

// Создание экземпляра TCP-сканера
func New(opts ...Option) (*Scanner, error) {
	cfg := scannerConfig{
		concurrency: defaultConcurrency,
		timeout:     defaultConnectTimeout,
		resolver:    net.DefaultResolver,
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	if cfg.resolver == nil {
		cfg.resolver = net.DefaultResolver
	}

	return &Scanner{
		concurrency: cfg.concurrency,
		timeout:     cfg.timeout,
		resolver:    cfg.resolver,
	}, nil
}

// Асинхронное TCP Connect-сканирование хостов и портов
func (s *Scanner) Scan(ctx context.Context, hosts []string, ports ...PortSpec) (<-chan Result, error) {
	if s == nil {
		return nil, ErrNilScanner
	}
	if ctx == nil {
		return nil, ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	spec, err := resolvePortSpec(ports)
	if err != nil {
		return nil, err
	}

	resolvedPorts, err := spec.ports()
	if err != nil {
		return nil, err
	}

	resolution, err := resolveTargets(ctx, s.resolver, hosts)
	if err != nil {
		return nil, err
	}

	if len(resolution.Targets) == 0 && len(resolution.Errors) > 0 {
		return nil, resolution.Errors[0].Err
	}

	workerCfg := workerConfig{
		Concurrency: s.concurrency,
		Dial: dialConfig{
			Dialer: &net.Dialer{
				Timeout: s.timeout,
			},
		},
	}

	ch := runWorkers(ctx, resolution.Targets, resolvedPorts, workerCfg)
	if len(resolution.Errors) == 0 {
		return ch, nil
	}

	out := make(chan Result, s.concurrency)
	go emitScanResults(ctx, out, ch, resolution.Errors, resolvedPorts)
	return out, nil
}

// Формирование единой спецификации портов из аргументов Scan
func resolvePortSpec(specs []PortSpec) (PortSpec, error) {
	if len(specs) == 0 {
		return nil, ErrEmptyPorts
	}
	for _, spec := range specs {
		if spec == nil {
			return nil, ErrNilPortSpec
		}
	}
	if len(specs) == 1 {
		return specs[0], nil
	}
	return Combine(specs...), nil
}

// Объединение результатов проверки портов и ошибок разрешения хостов в единый канал
func emitScanResults(
	ctx context.Context,
	out chan<- Result,
	workerCh <-chan Result,
	resolveErrs []targetResolveError,
	ports []uint16,
) {
	defer close(out)

	for _, re := range resolveErrs {
		state := classifyError(ctx, re.Err)
		for _, port := range ports {
			if ctx.Err() != nil {
				return
			}
			res := Result{
				Host:  re.Host,
				Port:  port,
				State: state,
				Err:   re.Err,
			}
			select {
			case out <- res:
			case <-ctx.Done():
				return
			}
		}
	}

	for res := range workerCh {
		select {
		case out <- res:
		case <-ctx.Done():
			return
		}
	}
}
