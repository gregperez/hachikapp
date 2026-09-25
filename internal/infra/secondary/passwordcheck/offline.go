package passwordcheck

import (
	"context"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Offline stores normalized blocked passwords in process memory and performs no
// network calls.
type Offline struct {
	blocked map[string]struct{}
}

// NewOffline constructs an immutable blocklist checker.
func NewOffline(passwords []string) *Offline {
	blocked := make(map[string]struct{}, len(passwords))
	for _, password := range passwords {
		if normalized := normalize(password); normalized != "" {
			blocked[normalized] = struct{}{}
		}
	}
	return &Offline{blocked: blocked}
}

// Blocked reports whether the complete normalized password is listed.
func (o *Offline) Blocked(ctx context.Context, password string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if o == nil {
		return false, nil
	}
	_, blocked := o.blocked[normalize(password)]
	return blocked, nil
}

func normalize(value string) string {
	return strings.ToLower(norm.NFKC.String(strings.TrimSpace(value)))
}
