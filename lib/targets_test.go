package lib

import (
	"context"
	"errors"
	"net"
	"testing"
)

// Разбор и разрешение корректных IP и DNS-целей
func TestResolveTargetsValid(t *testing.T) {
	res, err := resolveTargets(context.Background(), net.DefaultResolver, []string{
		" 127.0.0.1 ",
		"127.0.0.1",
		"[::1]",
		"::1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("unexpected resolve errors: %v", res.Errors)
	}
	if len(res.Targets) != 3 {
		t.Fatalf("got %d targets (%+v), want 3", len(res.Targets), res.Targets)
	}
	if res.Targets[0].Host != "127.0.0.1" || !res.Targets[0].IP.Equal(net.ParseIP("127.0.0.1")) {
		t.Fatalf("unexpected first target: %+v", res.Targets[0])
	}
	if res.Targets[1].Host != "[::1]" || !res.Targets[1].IP.Equal(net.ParseIP("::1")) {
		t.Fatalf("unexpected second target: %+v", res.Targets[1])
	}
	if res.Targets[2].Host != "::1" || !res.Targets[2].IP.Equal(net.ParseIP("::1")) {
		t.Fatalf("unexpected third target: %+v", res.Targets[2])
	}
}

// Разрешение DNS с несколькими IP и частичный успех при смешанном списке целей
func TestResolveTargetsMultipleIPsAndPartialFailure(t *testing.T) {
	res, err := resolveTargets(context.Background(), net.DefaultResolver, []string{
		"127.0.0.1",
		"localhost",
		"nonexistent.invalid.local",
	})
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}
	if len(res.Targets) < 1 {
		t.Fatalf("expected at least 1 resolved target, got %+v", res.Targets)
	}

	seen := make(map[string]struct{}, len(res.Targets))
	for _, target := range res.Targets {
		if target.Host == "" || len(target.IP) == 0 {
			t.Fatalf("invalid resolved target: %+v", target)
		}
		key := target.Host + "|" + target.IP.String()
		if _, exists := seen[key]; exists {
			t.Fatalf("duplicate resolved target: %+v", target)
		}
		seen[key] = struct{}{}
	}

	if len(res.Errors) != 1 {
		t.Fatalf("expected 1 resolve error, got %d (%+v)", len(res.Errors), res.Errors)
	}
	if res.Errors[0].Host != "nonexistent.invalid.local" {
		t.Fatalf("got error host %q, want %q", res.Errors[0].Host, "nonexistent.invalid.local")
	}
	if !errors.Is(res.Errors[0].Err, ErrResolveFailure) {
		t.Fatalf("got error %v, want %v", res.Errors[0].Err, ErrResolveFailure)
	}
}

// Ошибки пустых и некорректных целей
func TestResolveTargetsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		hosts   []string
		wantErr error
	}{
		{
			name:    "empty hosts slice",
			hosts:   nil,
			wantErr: ErrEmptyHosts,
		},
		{
			name:    "empty string host",
			hosts:   []string{""},
			wantErr: ErrEmptyHost,
		},
		{
			name:    "whitespace only host",
			hosts:   []string{"   \t  "},
			wantErr: ErrEmptyHost,
		},
		{
			name:    "invalid hostname with slash",
			hosts:   []string{"example.com/path"},
			wantErr: ErrInvalidTarget,
		},
		{
			name:    "invalid hostname with leading hyphen",
			hosts:   []string{"-bad-host.example.com"},
			wantErr: ErrInvalidTarget,
		},
		{
			name:    "zone id rejected in ip target",
			hosts:   []string{"fe80::1%lo0"},
			wantErr: ErrInvalidTarget,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveTargets(context.Background(), net.DefaultResolver, tc.hosts)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("got error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// Отмена контекста при разрешении целей
func TestResolveTargetsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := resolveTargets(ctx, net.DefaultResolver, []string{"127.0.0.1"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got error %v, want %v", err, context.Canceled)
	}
}

// Неразрешимое доменное имя в списке целей
func TestResolveTargetsUnresolvableHost(t *testing.T) {
	res, err := resolveTargets(context.Background(), net.DefaultResolver, []string{"nonexistent.invalid.local"})
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}
	if len(res.Targets) != 0 {
		t.Fatalf("expected no resolved targets, got %+v", res.Targets)
	}
	if len(res.Errors) != 1 {
		t.Fatalf("expected 1 resolve error, got %d", len(res.Errors))
	}
	if res.Errors[0].Host != "nonexistent.invalid.local" {
		t.Fatalf("got host %q, want %q", res.Errors[0].Host, "nonexistent.invalid.local")
	}
	if !errors.Is(res.Errors[0].Err, ErrResolveFailure) {
		t.Fatalf("got error %v, want wrapped %v", res.Errors[0].Err, ErrResolveFailure)
	}
}
