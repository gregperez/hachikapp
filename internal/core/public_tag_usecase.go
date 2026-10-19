package core

import (
	"context"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

// PublicTagUseCase serves the no-login public tag page and the owner's scan history.
type PublicTagUseCase interface {
	View(ctx context.Context, code string) (domain.PublicTagView, error)
	Photo(ctx context.Context, code string) (domain.PetPhoto, error)
	RecordScan(ctx context.Context, code, visitorID, ip, userAgent string) (domain.ScanReceipt, error)
	SendMessage(ctx context.Context, scanID, token string, in domain.FinderMessageInput) error
	RevealContact(ctx context.Context, scanID, token string) (domain.PublicContact, error)
	ListPetScans(ctx context.Context, ownerID, petID string, page authBaseDomain.PageRequest) (authBaseDomain.Page[domain.ScanWithMessages], error)
}
