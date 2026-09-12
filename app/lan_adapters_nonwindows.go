//go:build !windows

package main

import (
	"net"
	"net/netip"
)

func platformLANAdapters() ([]lanAdapter, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var result []lanAdapter
	for _, adapter := range interfaces {
		if adapter.Flags&net.FlagUp == 0 || adapter.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := adapter.Addrs()
		if err != nil {
			continue
		}
		for _, value := range addresses {
			prefix, err := netip.ParsePrefix(value.String())
			if err == nil && prefix.Addr().Is4() {
				result = append(result, lanAdapter{Name: adapter.Name, Address: prefix.Addr().String()})
			}
		}
	}
	return result, nil
}
