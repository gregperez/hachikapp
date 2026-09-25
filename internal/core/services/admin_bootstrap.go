package services

import (
	"context"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
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
// Persistence must serialize concurrent bootstraps (advisory lock inside SaveFirstAdmin).
func (b *AdminBootstrap) Run(ctx context.Context, name, email, password string) error {
	if name == "" || email == "" {
		return domain.ErrWeakPassword
	}
	if err := domain.ValidatePassword(password); err != nil {
		return err
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
	return b.users.SaveFirstAdmin(ctx, user)
}
