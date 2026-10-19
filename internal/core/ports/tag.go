package ports

import (
	"context"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

// TagPort persists pet tags and their generation batches.
type TagPort interface {
	// CreateBatch stores the batch and its codes atomically. It returns
	// domain.ErrDuplicateTagCode when a code already exists.
	CreateBatch(ctx context.Context, batch domain.TagBatch, codes []string) (domain.TagBatch, error)
	ListBatches(ctx context.Context, page authBaseDomain.PageRequest) (authBaseDomain.Page[domain.TagBatch], error)
	BatchTags(ctx context.Context, batchID string) ([]domain.PetTag, error)
	// GetByCode returns domain.ErrNotFound for an unknown code.
	GetByCode(ctx context.Context, code string) (domain.PetTag, error)
	// GetByID returns domain.ErrNotFound for an unknown (or malformed) tag id.
	GetByID(ctx context.Context, tagID string) (domain.PetTag, error)
	// ListByPet returns the pet's tags, newest first, only when the pet belongs to ownerID.
	ListByPet(ctx context.Context, ownerID, petID string) ([]domain.PetTag, error)
	// Activate links an unassigned tag (ownerless or reserved for ownerID) to a
	// pet atomically with the maxActive check. It returns domain.ErrNotFound when
	// the pet is not ownerID's, domain.ErrTagLimitReached when the pet already has
	// maxActive active tags and domain.ErrTagNotAssignable when no row qualifies.
	Activate(ctx context.Context, code, ownerID, petID string, maxActive int, at time.Time) (domain.PetTag, error)
	// CreateDigital creates an already active digital tag under the same atomic
	// ownership and maxActive checks as Activate. It returns
	// domain.ErrDuplicateTagCode on a code collision.
	CreateDigital(ctx context.Context, ownerID, petID, code string, maxActive int, at time.Time) (domain.PetTag, error)
	// Disable disables an active tag of that owner and pet, else domain.ErrNotFound.
	Disable(ctx context.Context, ownerID, petID, tagID string, at time.Time) (domain.PetTag, error)
}

// TagScanPort persists public tag scans and the messages finders leave.
type TagScanPort interface {
	// RecordScan stores scan, unless the same (tag, visitor) already scanned at or
	// after dedupeSince: then that scan keeps its time, takes the new
	// scan_token_hash and is returned with reused=true.
	RecordScan(ctx context.Context, scan domain.TagScan, dedupeSince time.Time) (domain.TagScan, bool, error)
	// GetScan returns domain.ErrNotFound for an unknown scan.
	GetScan(ctx context.Context, scanID string) (domain.TagScan, error)
	AddMessage(ctx context.Context, msg domain.TagMessage) (domain.TagMessage, error)
	// MarkPhoneRevealed returns true only for the call that sets phone_revealed_at,
	// and domain.ErrNotFound when the scan does not exist.
	MarkPhoneRevealed(ctx context.Context, scanID string, at time.Time) (bool, error)
	// ListScansByPet returns the pet's scans newest first with their messages
	// (oldest first), only when the pet belongs to ownerID; otherwise an empty page.
	// The cursor key is (scanned_at, id).
	ListScansByPet(ctx context.Context, ownerID, petID string, page authBaseDomain.PageRequest) (authBaseDomain.Page[domain.ScanWithMessages], error)
}
