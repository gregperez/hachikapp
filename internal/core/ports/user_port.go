package ports

import (
	"context"

	"github.com/WilsonSayago/authBase/v3/core/port"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
)

// UserPort is the application persistence port for users.
// It also satisfies authBase UserReader and CredentialReader.
type UserPort interface {
	port.UserReader[domain.User]
	port.CredentialReader
	Save(ctx context.Context, u domain.User) error
	FindAll(ctx context.Context, pageSize, offset int) ([]domain.User, int, error)
	FindById(ctx context.Context, id string) (domain.User, error)
	FindByEmail(ctx context.Context, email string) (domain.User, error)
	FindFullById(ctx context.Context, id string) (domain.User, error)
	Update(ctx context.Context, u domain.User) error
	SetActive(ctx context.Context, id string, active bool) error
	CountAdmins(ctx context.Context) (int, error)
	SaveFirstAdmin(ctx context.Context, u domain.User) error
	DemoteAdminIfNotLast(ctx context.Context, id string) error
	DeactivateAdminIfNotLast(ctx context.Context, id string) error
	UpdatePassword(ctx context.Context, id, passwordHash string) error
	FindRoleIDByName(ctx context.Context, name string) (string, error)
}
