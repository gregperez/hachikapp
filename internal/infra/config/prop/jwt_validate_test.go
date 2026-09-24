package prop

import (
	"strings"
	"testing"

	"github.com/WilsonSayago/authBase/v3/infra/config/properties"
)

func TestJwtRejectsInvalidStartupConfig(t *testing.T) {
	t.Parallel()
	base := properties.Jwt{
		SecretKey:        strings.Repeat("a", 32),
		RefreshSecret:    strings.Repeat("b", 32),
		ExpirationTime:   1,
		RefreshTokenTime: 24,
		Issuer:           "iss",
		Audience:         "aud",
		LeewaySeconds:    0,
	}

	cases := []struct {
		name string
		mut  func(*properties.Jwt)
	}{
		{"short access", func(j *properties.Jwt) { j.SecretKey = "short" }},
		{"equal secrets", func(j *properties.Jwt) { j.RefreshSecret = j.SecretKey }},
		{"empty issuer", func(j *properties.Jwt) { j.Issuer = "" }},
		{"refresh ttl <= access", func(j *properties.Jwt) { j.RefreshTokenTime = 1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			j := base
			tc.mut(&j)
			if err := j.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
