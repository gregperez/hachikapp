package controller

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ClientOriginResolver derives an origin without trusting forwarding headers by
// default. Forwarded addresses are considered only behind configured proxies.
type ClientOriginResolver struct {
	trusted []netip.Prefix
}

func NewClientOriginResolver(trustedCIDRs []string) (*ClientOriginResolver, error) {
	trusted := make([]netip.Prefix, 0, len(trustedCIDRs))
	for _, raw := range trustedCIDRs {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil {
			return nil, err
		}
		trusted = append(trusted, prefix)
	}
	return &ClientOriginResolver{trusted: trusted}, nil
}

func (r *ClientOriginResolver) Origin(request *http.Request) string {
	peer := remoteIP(request.RemoteAddr)
	if !r.isTrusted(peer) {
		return originString(peer)
	}

	forwarded := strings.Split(request.Header.Get("X-Forwarded-For"), ",")
	for i := len(forwarded) - 1; i >= 0; i-- {
		address, err := netip.ParseAddr(strings.TrimSpace(forwarded[i]))
		if err != nil {
			continue
		}
		address = address.Unmap()
		if !r.isTrusted(address) {
			return address.String()
		}
	}
	return originString(peer)
}

func (r *ClientOriginResolver) isTrusted(address netip.Addr) bool {
	if !address.IsValid() {
		return false
	}
	for _, prefix := range r.trusted {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func remoteIP(remoteAddress string) netip.Addr {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err != nil {
		host = remoteAddress
	}
	address, err := netip.ParseAddr(strings.TrimSpace(host))
	if err != nil {
		return netip.Addr{}
	}
	return address.Unmap()
}

func originString(address netip.Addr) string {
	if !address.IsValid() {
		return "unknown"
	}
	return address.String()
}
