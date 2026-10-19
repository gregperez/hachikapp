package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
)

// maxTagCodeAttempts bounds how often a code collision is retried.
const maxTagCodeAttempts = 3

// TagService implements the tag use cases of owners and admins.
type TagService struct {
	tags   ports.TagPort
	pets   ports.PetPort
	random io.Reader
	now    func() time.Time
}

func NewTagService(tags ports.TagPort, pets ports.PetPort, random io.Reader) *TagService {
	return &TagService{
		tags:   tags,
		pets:   pets,
		random: random,
		now:    func() time.Time { return time.Now().UTC() },
	}
}

var _ core.TagUseCase = (*TagService)(nil)

func (s *TagService) CreateBatch(ctx context.Context, actorID, label string, quantity int) (domain.TagBatch, error) {
	if err := domain.ValidateTagBatch(label, quantity); err != nil {
		return domain.TagBatch{}, err
	}
	batch := domain.TagBatch{Label: strings.TrimSpace(label), Quantity: quantity, CreatedBy: actorID}
	var lastErr error
	for attempt := 0; attempt < maxTagCodeAttempts; attempt++ {
		codes, err := s.newCodes(quantity)
		if err != nil {
			return domain.TagBatch{}, err
		}
		created, err := s.tags.CreateBatch(ctx, batch, codes)
		if err == nil {
			return created, nil
		}
		if !errors.Is(err, domain.ErrDuplicateTagCode) {
			return domain.TagBatch{}, fmt.Errorf("create tag batch: %w", err)
		}
		lastErr = err
	}
	return domain.TagBatch{}, lastErr
}

func (s *TagService) ListBatches(ctx context.Context, page authBaseDomain.PageRequest) (authBaseDomain.Page[domain.TagBatch], error) {
	return s.tags.ListBatches(ctx, page)
}

func (s *TagService) BatchTags(ctx context.Context, batchID string) ([]domain.PetTag, error) {
	if !isPetID(batchID) {
		return nil, domain.ErrNotFound
	}
	return s.tags.BatchTags(ctx, batchID)
}

func (s *TagService) ListPetTags(ctx context.Context, ownerID, petID string) ([]domain.PetTag, error) {
	if err := s.ownedPet(ctx, ownerID, petID); err != nil {
		return nil, err
	}
	return s.tags.ListByPet(ctx, ownerID, petID)
}

func (s *TagService) CreateDigital(ctx context.Context, ownerID, petID string) (domain.PetTag, error) {
	if err := s.ownedPet(ctx, ownerID, petID); err != nil {
		return domain.PetTag{}, err
	}
	var lastErr error
	for attempt := 0; attempt < maxTagCodeAttempts; attempt++ {
		code, err := domain.NewTagCode(s.random)
		if err != nil {
			return domain.PetTag{}, fmt.Errorf("generate tag code: %w", err)
		}
		tag, err := s.tags.CreateDigital(ctx, ownerID, petID, code, domain.MaxActiveTagsPerPet, s.now())
		if err == nil {
			return tag, nil
		}
		if !errors.Is(err, domain.ErrDuplicateTagCode) {
			return domain.PetTag{}, err
		}
		lastErr = err
	}
	return domain.PetTag{}, lastErr
}

func (s *TagService) Activate(ctx context.Context, ownerID, code, petID string) (domain.PetTag, error) {
	normalized, ok := domain.NormalizeTagCode(code)
	if !ok {
		return domain.PetTag{}, domain.ErrNotFound
	}
	if err := s.ownedPet(ctx, ownerID, petID); err != nil {
		return domain.PetTag{}, err
	}
	return s.tags.Activate(ctx, normalized, ownerID, petID, domain.MaxActiveTagsPerPet, s.now())
}

func (s *TagService) Disable(ctx context.Context, ownerID, petID, tagID string) (domain.PetTag, error) {
	if strings.TrimSpace(ownerID) == "" {
		return domain.PetTag{}, domain.ErrForbidden
	}
	if !isPetID(petID) || !isPetID(tagID) {
		return domain.PetTag{}, domain.ErrNotFound
	}
	return s.tags.Disable(ctx, ownerID, petID, tagID, s.now())
}

// ownedPet reports domain.ErrNotFound unless petID is a pet of ownerID.
func (s *TagService) ownedPet(ctx context.Context, ownerID, petID string) error {
	if strings.TrimSpace(ownerID) == "" {
		return domain.ErrForbidden
	}
	if !isPetID(petID) {
		return domain.ErrNotFound
	}
	if _, err := s.pets.GetForOwner(ctx, ownerID, petID); err != nil {
		return err
	}
	return nil
}

// newCodes returns n distinct tag codes.
func (s *TagService) newCodes(n int) ([]string, error) {
	seen := make(map[string]struct{}, n)
	codes := make([]string, 0, n)
	for len(codes) < n {
		code, err := domain.NewTagCode(s.random)
		if err != nil {
			return nil, fmt.Errorf("generate tag code: %w", err)
		}
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}
	return codes, nil
}
