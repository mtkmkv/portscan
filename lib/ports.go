package lib

import (
	"errors"
	"sort"
)

var (
	ErrEmptyPorts   = errors.New("empty ports")
	ErrInvalidPort  = errors.New("invalid port")
	ErrInvalidRange = errors.New("invalid port range")
)

// Интерфейс набора TCP-портов
type PortSpec interface {
	ports() ([]uint16, error)
}

type portListSpec struct {
	values []uint16
}

// Набор отдельных TCP-портов
func Ports(ports ...uint16) PortSpec {
	copied := make([]uint16, len(ports))
	copy(copied, ports)
	return portListSpec{values: copied}
}

func (s portListSpec) ports() ([]uint16, error) {
	if len(s.values) == 0 {
		return nil, ErrEmptyPorts
	}
	for _, p := range s.values {
		if p == 0 {
			return nil, ErrInvalidPort
		}
	}
	return normalizePorts(s.values), nil
}

type portRangeSpec struct {
	start uint16
	end   uint16
}

// Диапазон TCP-портов включительно
func Range(start, end uint16) PortSpec {
	return portRangeSpec{start: start, end: end}
}

func (s portRangeSpec) ports() ([]uint16, error) {
	if s.start == 0 || s.end == 0 {
		return nil, ErrInvalidPort
	}
	if s.start > s.end {
		return nil, ErrInvalidRange
	}
	count := int(s.end) - int(s.start) + 1
	res := make([]uint16, count)
	for i := range count {
		res[i] = uint16(int(s.start) + i)
	}
	return res, nil
}

type combinedPortsSpec struct {
	specs []PortSpec
}

// Объединение нескольких спецификаций портов
func Combine(specs ...PortSpec) PortSpec {
	copied := make([]PortSpec, len(specs))
	copy(copied, specs)
	return combinedPortsSpec{specs: copied}
}

func (s combinedPortsSpec) ports() ([]uint16, error) {
	if len(s.specs) == 0 {
		return nil, ErrEmptyPorts
	}
	var merged []uint16
	for _, spec := range s.specs {
		if spec == nil {
			continue
		}
		list, err := spec.ports()
		if err != nil {
			return nil, err
		}
		merged = mergeSortedPorts(merged, list)
	}
	if len(merged) == 0 {
		return nil, ErrEmptyPorts
	}
	return merged, nil
}

// Сортировка и удаление дубликатов портов
func normalizePorts(raw []uint16) []uint16 {
	copied := make([]uint16, len(raw))
	copy(copied, raw)
	sort.Slice(copied, func(i, j int) bool {
		return copied[i] < copied[j]
	})
	out := copied[:0]
	for _, p := range copied {
		if len(out) == 0 || out[len(out)-1] != p {
			out = append(out, p)
		}
	}
	return out
}

// Слияние двух отсортированных списков портов без дубликатов
func mergeSortedPorts(a, b []uint16) []uint16 {
	res := make([]uint16, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		var next uint16
		if a[i] < b[j] {
			next = a[i]
			i++
		} else if b[j] < a[i] {
			next = b[j]
			j++
		} else {
			next = a[i]
			i++
			j++
		}
		if len(res) == 0 || res[len(res)-1] != next {
			res = append(res, next)
		}
	}
	for ; i < len(a); i++ {
		if len(res) == 0 || res[len(res)-1] != a[i] {
			res = append(res, a[i])
		}
	}
	for ; j < len(b); j++ {
		if len(res) == 0 || res[len(res)-1] != b[j] {
			res = append(res, b[j])
		}
	}
	return res
}
