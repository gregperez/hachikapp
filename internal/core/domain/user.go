package domain

import authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"

// User is the local identity model. It embeds authBase UserGeneric and never
// exposes password hashes through IUserGeneric.
type User struct {
	authBaseDomain.UserGeneric
	// plainPassword is write-only material for create flows; empty on reads.
	plainPassword string
}

// WithPlainPassword attaches a one-shot plaintext password for persistence hashing.
func WithPlainPassword(u User, password string) User {
	u.plainPassword = password
	return u
}

// PlainPassword returns the write-only password, if any.
func (u User) PlainPassword() string {
	return u.plainPassword
}
