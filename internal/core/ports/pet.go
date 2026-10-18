package ports

import (
	"context"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

// PetPort persists pet records. Every method is scoped to the owning account:
// a pet that belongs to someone else behaves as domain.ErrNotFound.
type PetPort interface {
	ListByOwner(ctx context.Context, ownerID string, limit int) ([]domain.Pet, error)
	CountByOwner(ctx context.Context, ownerID string) (int, error)
	GetForOwner(ctx context.Context, ownerID, petID string) (domain.Pet, error)
	// Create returns domain.ErrDuplicateChip when the chip number is taken.
	Create(ctx context.Context, ownerID string, details domain.PetDetails) (domain.Pet, error)
	// Update returns domain.ErrStaleVersion when expectedVersion is outdated.
	Update(ctx context.Context, ownerID, petID string, expectedVersion int64, details domain.PetDetails) (domain.Pet, error)
	// SetPhotoUpdatedAt records photo metadata; nil means the pet has no photo.
	SetPhotoUpdatedAt(ctx context.Context, ownerID, petID string, at *time.Time) (domain.Pet, error)
	Delete(ctx context.Context, ownerID, petID string) error
}

// PetPhotoPort stores pet image bytes. The PostgreSQL adapter keeps them in
// the database; an object-storage adapter (S3/GCS) can replace it without
// touching the use cases.
type PetPhotoPort interface {
	Get(ctx context.Context, petID string) (domain.PetPhoto, error)
	Put(ctx context.Context, petID string, photo domain.PetPhoto) error
	// Delete is idempotent: a missing photo is not an error.
	Delete(ctx context.Context, petID string) error
}
