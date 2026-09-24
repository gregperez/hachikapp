package services

import (
	"context"

	authBaseDomain "github.com/WilsonSayago/authBase/v3/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/ports"
)

const adminRoleName = "admin"

// AdminBootstrap creates the first administrator exactly once.
type AdminBootstrap struct {
	users ports.UserPort
}

func NewAdminBootstrap(users ports.UserPort) *AdminBootstrap {
	return &AdminBootstrap{users: users}
}

// Run creates an administrator with the admin role. Fails if any admin exists.
func (b *AdminBootstrap) Run(ctx context.Context, name, email, password string) error {
	if name == "" || email == "" {
		return domain.ErrWeakPassword
	}
	if err := domain.ValidatePassword(password); err != nil {
		return err
	}
	n, err := b.users.CountAdmins(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return domain.ErrAdminExists
	}
	roleID, err := b.users.FindRoleIDByName(ctx, adminRoleName)
	if err != nil {
		return err
	}
	user := domain.WithPlainPassword(domain.User{
		UserGeneric: authBaseDomain.NewUserGeneric(
			"",
			name,
			email,
			[]authBaseDomain.Role{{Base: authBaseDomain.Base{Id: roleID, Active: true}}},
			true,
			true,
		),
	}, password)
	return b.users.Save(ctx, user)
}
