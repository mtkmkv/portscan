package lib

import (
	"errors"
	"slices"
	"testing"
)

// Проверка наборов портов, диапазонов и объединений
func TestPortSpecValid(t *testing.T) {
	tests := []struct {
		name string
		spec PortSpec
		want []uint16
	}{
		{
			name: "single ports with duplicates and unsorted order",
			spec: Ports(443, 80, 443, 22, 80),
			want: []uint16{22, 80, 443},
		},
		{
			name: "max port number",
			spec: Ports(65535),
			want: []uint16{65535},
		},
		{
			name: "inclusive port range",
			spec: Range(8000, 8003),
			want: []uint16{8000, 8001, 8002, 8003},
		},
		{
			name: "single port range",
			spec: Range(80, 80),
			want: []uint16{80},
		},
		{
			name: "range up to max port",
			spec: Range(65533, 65535),
			want: []uint16{65533, 65534, 65535},
		},
		{
			name: "combine overlapping specs",
			spec: Combine(Range(80, 82), Ports(82, 443, 80)),
			want: []uint16{80, 81, 82, 443},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.spec.ports()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// Проверка изоляции срезов при повторных вызовах и внешней мутации
func TestPortSpecSliceIsolation(t *testing.T) {
	raw := []uint16{8080, 80, 8080}
	listSpec := Ports(raw...)
	raw[0] = 0
	raw[1] = 0

	specs := []struct {
		name string
		spec PortSpec
		want []uint16
	}{
		{
			name: "ports list",
			spec: listSpec,
			want: []uint16{80, 8080},
		},
		{
			name: "port range",
			spec: Range(80, 82),
			want: []uint16{80, 81, 82},
		},
		{
			name: "combined spec",
			spec: Combine(Range(80, 81), Ports(81, 443)),
			want: []uint16{80, 81, 443},
		},
	}

	for _, tc := range specs {
		t.Run(tc.name, func(t *testing.T) {
			first, err := tc.spec.ports()
			if err != nil {
				t.Fatalf("unexpected error on first call: %v", err)
			}
			if !slices.Equal(first, tc.want) {
				t.Fatalf("first call got %v, want %v", first, tc.want)
			}

			for i := range first {
				first[i] = 65535
			}

			second, err := tc.spec.ports()
			if err != nil {
				t.Fatalf("unexpected error on second call: %v", err)
			}
			if !slices.Equal(second, tc.want) {
				t.Fatalf("second call got %v, want %v", second, tc.want)
			}
		})
	}
}

// Проверка ошибочных и граничных спецификаций портов
func TestPortSpecErrors(t *testing.T) {
	tests := []struct {
		name    string
		spec    PortSpec
		wantErr error
	}{
		{
			name:    "empty port list",
			spec:    Ports(),
			wantErr: ErrEmptyPorts,
		},
		{
			name:    "zero port in list",
			spec:    Ports(80, 0),
			wantErr: ErrInvalidPort,
		},
		{
			name:    "zero start in range",
			spec:    Range(0, 10),
			wantErr: ErrInvalidPort,
		},
		{
			name:    "zero end in range",
			spec:    Range(1, 0),
			wantErr: ErrInvalidPort,
		},
		{
			name:    "reverse range",
			spec:    Range(1000, 1),
			wantErr: ErrInvalidRange,
		},
		{
			name:    "empty combine",
			spec:    Combine(),
			wantErr: ErrEmptyPorts,
		},
		{
			name:    "combine with nil specs only",
			spec:    Combine(nil, nil),
			wantErr: ErrEmptyPorts,
		},
		{
			name:    "combine returns error from invalid spec",
			spec:    Combine(Ports(80), Range(500, 100)),
			wantErr: ErrInvalidRange,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.spec.ports()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("got error %v, want %v", err, tc.wantErr)
			}
		})
	}
}
