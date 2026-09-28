package sessionmeta

import "testing"

func TestTruncateIPMasksIPv4AndIPv6(t *testing.T) {
	t.Parallel()
	if got := TruncateIP("192.168.10.45"); got != "192.168.10.0" {
		t.Fatalf("ipv4=%q", got)
	}
	if got := TruncateIP("2001:db8:abcd:1234:5678::1"); got != "2001:db8:abcd:1234::" {
		t.Fatalf("ipv6=%q", got)
	}
	if got := TruncateIP("unknown"); got != "" {
		t.Fatalf("invalid=%q", got)
	}
}

func TestSummarizeUserAgentStripsControlsAndLimitsLength(t *testing.T) {
	t.Parallel()
	if got := SummarizeUserAgent("Mozilla/5.0"); got != "Mozilla/5.0" {
		t.Fatalf("got=%q", got)
	}
	if got := SummarizeUserAgent(""); got != "Cliente desconocido" {
		t.Fatalf("empty=%q", got)
	}
	long := stringsRepeat("a", 200)
	if got := SummarizeUserAgent(long); len([]rune(got)) != 80 {
		t.Fatalf("len=%d", len([]rune(got)))
	}
}

func stringsRepeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
