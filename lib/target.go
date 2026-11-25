package lib

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
)

var (
	ErrEmptyHosts     = errors.New("empty hosts")
	ErrEmptyHost      = errors.New("empty host")
	ErrInvalidTarget  = errors.New("invalid target")
	ErrResolveFailure = errors.New("resolve failure")
)

// Разрешённая цель для проверки порта
type resolvedTarget struct {
	Host string
	IP   net.IP
}

// Ошибка разрешения конкретного хоста
type targetResolveError struct {
	Host string
	Err  error
}

// Итог разбора и разрешения списка хостов
type targetResolution struct {
	Targets []resolvedTarget
	Errors  []targetResolveError
}

// Разбор и разрешение списка хостов в цели и ошибки разрешения
func resolveTargets(ctx context.Context, resolver *net.Resolver, rawHosts []string) (targetResolution, error) {
	if len(rawHosts) == 0 {
		return targetResolution{}, ErrEmptyHosts
	}

	seenRaw := make(map[string]struct{}, len(rawHosts))
	seenResolved := make(map[string]struct{}, len(rawHosts))
	var out targetResolution

	for _, raw := range rawHosts {
		if err := ctx.Err(); err != nil {
			return targetResolution{}, err
		}

		host := strings.TrimSpace(raw)
		if host == "" {
			return targetResolution{}, ErrEmptyHost
		}
		if _, exists := seenRaw[host]; exists {
			continue
		}
		seenRaw[host] = struct{}{}

		if addr, ok := parseIPTarget(host); ok {
			key := host + "|" + addr.String()
			if _, exists := seenResolved[key]; !exists {
				seenResolved[key] = struct{}{}
				out.Targets = append(out.Targets, resolvedTarget{
					Host: host,
					IP:   cloneIP(addr),
				})
			}
			continue
		}

		if !isValidHostname(host) {
			return targetResolution{}, ErrInvalidTarget
		}

		activeResolver := resolver
		if activeResolver == nil {
			activeResolver = net.DefaultResolver
		}

		addrs, err := activeResolver.LookupIPAddr(ctx, host)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return targetResolution{}, ctxErr
			}
			out.Errors = append(out.Errors, targetResolveError{
				Host: host,
				Err:  errors.Join(ErrResolveFailure, err),
			})
			continue
		}

		added := 0
		for _, item := range addrs {
			addr, ok := netip.AddrFromSlice(item.IP)
			if !ok {
				continue
			}
			addr = addr.Unmap()
			if addr.Zone() != "" {
				continue
			}
			key := host + "|" + addr.String()
			if _, exists := seenResolved[key]; exists {
				continue
			}
			seenResolved[key] = struct{}{}
			out.Targets = append(out.Targets, resolvedTarget{
				Host: host,
				IP:   cloneIP(addr),
			})
			added++
		}

		if added == 0 {
			out.Errors = append(out.Errors, targetResolveError{
				Host: host,
				Err:  ErrResolveFailure,
			})
		}
	}

	return out, nil
}

// Разбор IPv4 или IPv6 адреса с поддержкой квадратных скобок
func parseIPTarget(host string) (netip.Addr, bool) {
	raw := host
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") && len(raw) > 2 {
		raw = raw[1 : len(raw)-1]
	}
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return netip.Addr{}, false
	}
	addr = addr.Unmap()
	if addr.Zone() != "" {
		return netip.Addr{}, false
	}
	return addr, true
}

// Копирование IP-адреса в отдельный срез
func cloneIP(addr netip.Addr) net.IP {
	src := addr.AsSlice()
	return net.IP(bytes.Clone(src))
}

// Проверка синтаксиса доменного имени
func isValidHostname(host string) bool {
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	if strings.Contains(host, ":") || strings.Contains(host, "/") {
		return false
	}
	labels := strings.Split(strings.TrimSuffix(host, "."), ".")
	if len(labels) == 0 {
		return false
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := range len(label) {
			c := label[i]
			isAlpha := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
			isDigit := c >= '0' && c <= '9'
			isHyphen := c == '-'
			if !isAlpha && !isDigit && !isHyphen {
				return false
			}
		}
	}
	return true
}
