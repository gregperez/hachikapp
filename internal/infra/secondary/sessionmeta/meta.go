package sessionmeta

import (
	"context"
	"net/netip"
	"strings"
	"unicode"
	"unicode/utf8"
)

type ctxKey struct{}

// Meta is truncated client display data. It is never used as an auth factor.
type Meta struct {
	ClientLabel string
	IPPrefix    string
}

func With(ctx context.Context, meta Meta) context.Context {
	return context.WithValue(ctx, ctxKey{}, meta)
}

func From(ctx context.Context) (Meta, bool) {
	meta, ok := ctx.Value(ctxKey{}).(Meta)
	return meta, ok
}

func FromHTTP(userAgent, origin string) Meta {
	return Meta{
		ClientLabel: SummarizeUserAgent(userAgent),
		IPPrefix:    TruncateIP(origin),
	}
}

func SummarizeUserAgent(raw string) string {
	cleaned := strings.Map(func(r rune) rune {
		if r < 32 || r == 127 || !unicode.IsPrint(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(raw))
	if cleaned == "" {
		return "Cliente desconocido"
	}
	if utf8.RuneCountInString(cleaned) > 80 {
		runes := []rune(cleaned)
		cleaned = string(runes[:80])
	}
	return cleaned
}

func TruncateIP(raw string) string {
	address, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	address = address.Unmap()
	if address.Is4() {
		octets := address.As4()
		octets[3] = 0
		return netip.AddrFrom4(octets).String()
	}
	if address.Is6() {
		prefix, err := address.Prefix(64)
		if err != nil {
			return ""
		}
		return prefix.Addr().String()
	}
	return ""
}
