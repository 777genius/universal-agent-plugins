package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
)

// The native observer cannot inherit ambient proxies or send a native bearer
// through DNS, TLS redirection or a remote connection. HTTP redirects are not
// followed by RoundTrip; each adapter still binds its process-issued origin.
func nativeLoopbackTransport() http.RoundTripper {
	return &http.Transport{Proxy: nil, DisableCompression: true, MaxResponseHeaderBytes: 64 * 1024,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(address)
			if err != nil || network != "tcp" || host != "127.0.0.1" {
				return nil, fmt.Errorf("native observer requires literal loopback")
			}
			return (&net.Dialer{}).DialContext(ctx, network, address)
		}}
}
