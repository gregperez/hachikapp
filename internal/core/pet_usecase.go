package core

import (
	"context"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

// PetUseCase manages the pet records of the authenticated account.
type PetUseCase interface {
	List(ctx context.Context, ownerID string) ([]domain.Pet, error)
	Get(ctx context.Context, ownerID, petID string) (domain.Pet, error)
	Create(ctx context.Context, ownerID string, details domain.PetDetails) (domain.Pet, error)
	Update(ctx context.Context, ownerID, petID string, version int64, details domain.PetDetails) (domain.Pet, error)
	Delete(ctx context.Context, ownerID, petID string) error
	GetPhoto(ctx context.Context, ownerID, petID string) (domain.PetPhoto, error)
	PutPhoto(ctx context.Context, ownerID, petID string, data []byte) (domain.Pet, error)
	DeletePhoto(ctx context.Context, ownerID, petID string) (domain.Pet, error)
}
