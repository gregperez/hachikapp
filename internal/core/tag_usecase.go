package core

import (
	"context"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

// TagUseCase manages pet tags: batch generation for admins and activation for owners.
type TagUseCase interface {
	CreateBatch(ctx context.Context, actorID, label string, quantity int) (domain.TagBatch, error)
	ListBatches(ctx context.Context, page authBaseDomain.PageRequest) (authBaseDomain.Page[domain.TagBatch], error)
	BatchTags(ctx context.Context, batchID string) ([]domain.PetTag, error)
	ListPetTags(ctx context.Context, ownerID, petID string) ([]domain.PetTag, error)
	CreateDigital(ctx context.Context, ownerID, petID string) (domain.PetTag, error)
	Activate(ctx context.Context, ownerID, code, petID string) (domain.PetTag, error)
	Disable(ctx context.Context, ownerID, petID, tagID string) (domain.PetTag, error)
}
