package diagnostics

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"time"
)

// ValidTarget accepts a bare IP address or an ASCII/Punycode DNS name. Keep
// these rules in sync with ui/src/helper/diagnostics.js. Never accept CLI flags,
// URLs, ports, CIDRs, scoped IPv6 addresses, or whitespace (including at edges).
func ValidTarget(target string) bool {
	if target == "" || len(target) > 254 {
		return false
	}
	if addr, err := netip.ParseAddr(target); err == nil {
		return addr.Zone() == ""
	}
	name := strings.TrimSuffix(target, ".")
	if len(name) > 253 {
		return false
	}
	labels := strings.Split(name, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !asciiLetter(c) && (c < '0' || c > '9') && c != '-' {
				return false
			}
		}
	}
	// Do not reinterpret malformed IPv4 addresses as DNS names.
	return strings.ContainsFunc(labels[len(labels)-1], asciiLetter)
}

func asciiLetter(c rune) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func resolveTarget(ctx context.Context, target, family string) (netip.Addr, error) {
	if addr, err := netip.ParseAddr(target); err == nil {
		addr = addr.Unmap()
		if family == "4" && !addr.Is4() || family == "6" && !addr.Is6() {
			return netip.Addr{}, errors.New("target does not match the selected IP version")
		}
		return addr, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	network := "ip"
	if family != "auto" {
		network += family
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, network, target)
	if err != nil || len(addresses) == 0 {
		return netip.Addr{}, errors.New("cannot resolve target for the selected IP version")
	}
	// Prefer IPv4 in auto mode; IPv6 remains explicitly selectable.
	if family == "auto" {
		for _, addr := range addresses {
			if addr.Unmap().Is4() {
				return addr.Unmap(), nil
			}
		}
	}
	return addresses[0].Unmap(), nil
}
