package services

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
)

// PetService implements the owner-scoped pet record use cases.
type PetService struct {
	pets   ports.PetPort
	photos ports.PetPhotoPort
	now    func() time.Time
}

func NewPetService(pets ports.PetPort, photos ports.PetPhotoPort) *PetService {
	return &PetService{pets: pets, photos: photos, now: time.Now}
}

var _ core.PetUseCase = (*PetService)(nil)

func (s *PetService) List(ctx context.Context, ownerID string) ([]domain.Pet, error) {
	if err := s.ready(ownerID); err != nil {
		return nil, err
	}
	pets, err := s.pets.ListByOwner(ctx, ownerID, domain.MaxPetsPerOwner)
	if err != nil {
		return nil, fmt.Errorf("list pets: %w", err)
	}
	return pets, nil
}

func (s *PetService) Get(ctx context.Context, ownerID, petID string) (domain.Pet, error) {
	if err := s.ready(ownerID); err != nil {
		return domain.Pet{}, err
	}
	if !isPetID(petID) {
		return domain.Pet{}, domain.ErrNotFound
	}
	return s.pets.GetForOwner(ctx, ownerID, petID)
}

func (s *PetService) Create(ctx context.Context, ownerID string, details domain.PetDetails) (domain.Pet, error) {
	if err := s.ready(ownerID); err != nil {
		return domain.Pet{}, err
	}
	normalized, err := s.validate(details)
	if err != nil {
		return domain.Pet{}, err
	}
	count, err := s.pets.CountByOwner(ctx, ownerID)
	if err != nil {
		return domain.Pet{}, fmt.Errorf("count pets: %w", err)
	}
	if count >= domain.MaxPetsPerOwner {
		return domain.Pet{}, domain.ErrPetLimitReached
	}
	return s.pets.Create(ctx, ownerID, normalized)
}

func (s *PetService) Update(ctx context.Context, ownerID, petID string, version int64, details domain.PetDetails) (domain.Pet, error) {
	if err := s.ready(ownerID); err != nil {
		return domain.Pet{}, err
	}
	if !isPetID(petID) {
		return domain.Pet{}, domain.ErrNotFound
	}
	if version <= 0 {
		return domain.Pet{}, &domain.PetValidationError{Issues: []domain.PetFieldIssue{{Field: "version", Code: "required"}}}
	}
	stored, err := s.pets.GetForOwner(ctx, ownerID, petID)
	if err != nil {
		return domain.Pet{}, err
	}
	// Visibility keys left out of the request keep their stored value, never
	// the defaults: a partial edit must not make hidden data public again.
	details.PublicVisibility = mergeVisibility(stored.PublicVisibility, details.PublicVisibility)
	normalized, err := s.validate(details)
	if err != nil {
		return domain.Pet{}, err
	}
	return s.pets.Update(ctx, ownerID, petID, version, normalized)
}

func (s *PetService) Delete(ctx context.Context, ownerID, petID string) error {
	if err := s.ready(ownerID); err != nil {
		return err
	}
	if !isPetID(petID) {
		return domain.ErrNotFound
	}
	pet, err := s.pets.GetForOwner(ctx, ownerID, petID)
	if err != nil {
		return err
	}
	if pet.HasPhoto {
		if err := s.photos.Delete(ctx, pet.ID); err != nil {
			return fmt.Errorf("delete pet photo: %w", err)
		}
	}
	return s.pets.Delete(ctx, ownerID, petID)
}

func (s *PetService) GetPhoto(ctx context.Context, ownerID, petID string) (domain.PetPhoto, error) {
	pet, err := s.Get(ctx, ownerID, petID)
	if err != nil {
		return domain.PetPhoto{}, err
	}
	if !pet.HasPhoto {
		return domain.PetPhoto{}, domain.ErrNotFound
	}
	return s.photos.Get(ctx, pet.ID)
}

func (s *PetService) PutPhoto(ctx context.Context, ownerID, petID string, data []byte) (domain.Pet, error) {
	pet, err := s.Get(ctx, ownerID, petID)
	if err != nil {
		return domain.Pet{}, err
	}
	contentType, err := domain.DetectPetPhotoType(data, http.DetectContentType(data))
	if err != nil {
		return domain.Pet{}, err
	}
	at := s.now().UTC().Truncate(time.Microsecond)
	if err := s.photos.Put(ctx, pet.ID, domain.PetPhoto{ContentType: contentType, Data: data, UpdatedAt: at}); err != nil {
		return domain.Pet{}, fmt.Errorf("store pet photo: %w", err)
	}
	return s.pets.SetPhotoUpdatedAt(ctx, ownerID, pet.ID, &at)
}

func (s *PetService) DeletePhoto(ctx context.Context, ownerID, petID string) (domain.Pet, error) {
	pet, err := s.Get(ctx, ownerID, petID)
	if err != nil {
		return domain.Pet{}, err
	}
	if !pet.HasPhoto {
		return pet, nil
	}
	if err := s.photos.Delete(ctx, pet.ID); err != nil {
		return domain.Pet{}, fmt.Errorf("delete pet photo: %w", err)
	}
	return s.pets.SetPhotoUpdatedAt(ctx, ownerID, pet.ID, nil)
}

func (s *PetService) validate(details domain.PetDetails) (domain.PetDetails, error) {
	normalized := domain.NormalizePetDetails(details)
	if err := domain.ValidatePetDetails(normalized, s.now().UTC()); err != nil {
		return domain.PetDetails{}, err
	}
	return normalized, nil
}

func (s *PetService) ready(ownerID string) error {
	if s.pets == nil || s.photos == nil {
		return errors.New("pet stores are required")
	}
	if strings.TrimSpace(ownerID) == "" {
		return domain.ErrForbidden
	}
	return nil
}

func mergeVisibility(stored, sent map[domain.PetPublicField]domain.PetVisibility) map[domain.PetPublicField]domain.PetVisibility {
	merged := make(map[domain.PetPublicField]domain.PetVisibility, len(stored)+len(sent))
	for field, value := range stored {
		merged[field] = value
	}
	for field, value := range sent {
		merged[field] = value
	}
	return merged
}

// isPetID accepts canonical UUIDs so malformed ids never reach SQL casts.
func isPetID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	_, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return err == nil
}
