package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

const (
	ownerA = "aaaaaaaa-0000-4000-8000-000000000001"
	ownerB = "bbbbbbbb-0000-4000-8000-000000000002"
)

type memPets struct {
	pets map[string]domain.Pet
	seq  int
}

func newMemPets() *memPets { return &memPets{pets: map[string]domain.Pet{}} }

func (m *memPets) ListByOwner(_ context.Context, ownerID string, limit int) ([]domain.Pet, error) {
	out := []domain.Pet{}
	for _, pet := range m.pets {
		if pet.OwnerID == ownerID && len(out) < limit {
			out = append(out, pet)
		}
	}
	return out, nil
}

func (m *memPets) CountByOwner(_ context.Context, ownerID string) (int, error) {
	count := 0
	for _, pet := range m.pets {
		if pet.OwnerID == ownerID {
			count++
		}
	}
	return count, nil
}

func (m *memPets) GetForOwner(_ context.Context, ownerID, petID string) (domain.Pet, error) {
	pet, ok := m.pets[petID]
	if !ok || pet.OwnerID != ownerID {
		return domain.Pet{}, domain.ErrNotFound
	}
	return pet, nil
}

func (m *memPets) Create(_ context.Context, ownerID string, details domain.PetDetails) (domain.Pet, error) {
	for _, pet := range m.pets {
		if details.ChipNumber != "" && pet.ChipNumber == details.ChipNumber {
			return domain.Pet{}, domain.ErrDuplicateChip
		}
	}
	m.seq++
	pet := domain.Pet{ID: fmt.Sprintf("00000000-0000-4000-8000-%012d", m.seq), OwnerID: ownerID, PetDetails: details, Version: 1}
	m.pets[pet.ID] = pet
	return pet, nil
}

func (m *memPets) Update(ctx context.Context, ownerID, petID string, version int64, details domain.PetDetails) (domain.Pet, error) {
	pet, err := m.GetForOwner(ctx, ownerID, petID)
	if err != nil {
		return domain.Pet{}, err
	}
	if pet.Version != version {
		return domain.Pet{}, domain.ErrStaleVersion
	}
	pet.PetDetails = details
	pet.Version++
	m.pets[petID] = pet
	return pet, nil
}

func (m *memPets) SetPhotoUpdatedAt(ctx context.Context, ownerID, petID string, at *time.Time) (domain.Pet, error) {
	pet, err := m.GetForOwner(ctx, ownerID, petID)
	if err != nil {
		return domain.Pet{}, err
	}
	pet.HasPhoto = at != nil
	if at != nil {
		pet.PhotoUpdatedAt = *at
	}
	m.pets[petID] = pet
	return pet, nil
}

func (m *memPets) Delete(ctx context.Context, ownerID, petID string) error {
	if _, err := m.GetForOwner(ctx, ownerID, petID); err != nil {
		return err
	}
	delete(m.pets, petID)
	return nil
}

type memPhotos struct{ photos map[string]domain.PetPhoto }

func (m *memPhotos) Get(_ context.Context, petID string) (domain.PetPhoto, error) {
	photo, ok := m.photos[petID]
	if !ok {
		return domain.PetPhoto{}, domain.ErrNotFound
	}
	return photo, nil
}

func (m *memPhotos) Put(_ context.Context, petID string, photo domain.PetPhoto) error {
	m.photos[petID] = photo
	return nil
}

func (m *memPhotos) Delete(_ context.Context, petID string) error {
	delete(m.photos, petID)
	return nil
}

func newTestPetService() (*PetService, *memPets, *memPhotos) {
	pets := newMemPets()
	photos := &memPhotos{photos: map[string]domain.PetPhoto{}}
	svc := NewPetService(pets, photos)
	svc.now = func() time.Time { return time.Date(2026, 10, 18, 15, 0, 0, 0, time.UTC) }
	return svc, pets, photos
}

func sampleDetails() domain.PetDetails {
	return domain.PetDetails{
		Name:    " Luna ",
		Species: domain.PetSpeciesDog,
		Owner:   domain.PetOwnerContact{FullName: "Wilson", Phone: "+56 9 1234 5678"},
	}
}

var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 32)...)

func TestPetServiceCreateNormalizesAndScopesToOwner(t *testing.T) {
	svc, _, _ := newTestPetService()
	ctx := context.Background()

	pet, err := svc.Create(ctx, ownerA, sampleDetails())
	if err != nil {
		t.Fatal(err)
	}
	if pet.Name != "Luna" || pet.OwnerID != ownerA || pet.PublicVisibility[domain.PetPublicPhoto] != domain.PetVisibilityAlways {
		t.Fatalf("pet=%+v", pet)
	}
	if _, err := svc.Get(ctx, ownerB, pet.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other owner must get not found, got %v", err)
	}
	if err := svc.Delete(ctx, ownerB, pet.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other owner delete: %v", err)
	}
	list, err := svc.List(ctx, ownerB)
	if err != nil || len(list) != 0 {
		t.Fatalf("owner B list=%v err=%v", list, err)
	}
}

func TestPetServiceRejectsInvalidInput(t *testing.T) {
	svc, _, _ := newTestPetService()
	ctx := context.Background()

	bad := sampleDetails()
	bad.Owner.Phone = ""
	if _, err := svc.Create(ctx, ownerA, bad); !errors.Is(err, domain.ErrInvalidPet) {
		t.Fatalf("expected invalid pet, got %v", err)
	}
	if _, err := svc.Get(ctx, ownerA, "not-a-uuid"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("malformed id must be not found, got %v", err)
	}
	if _, err := svc.Create(ctx, "", sampleDetails()); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("anonymous create: %v", err)
	}
}

func TestPetServiceUpdateRequiresCurrentVersion(t *testing.T) {
	svc, _, _ := newTestPetService()
	ctx := context.Background()
	pet, err := svc.Create(ctx, ownerA, sampleDetails())
	if err != nil {
		t.Fatal(err)
	}
	next := sampleDetails()
	next.Breed = "Quiltro"
	if _, err := svc.Update(ctx, ownerA, pet.ID, 0, next); !errors.Is(err, domain.ErrInvalidPet) {
		t.Fatalf("missing version: %v", err)
	}
	updated, err := svc.Update(ctx, ownerA, pet.ID, pet.Version, next)
	if err != nil || updated.Breed != "Quiltro" || updated.Version != 2 {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	if _, err := svc.Update(ctx, ownerA, pet.ID, pet.Version, next); !errors.Is(err, domain.ErrStaleVersion) {
		t.Fatalf("stale update: %v", err)
	}
}

func TestPetServiceEnforcesPetLimit(t *testing.T) {
	svc, pets, _ := newTestPetService()
	ctx := context.Background()
	for i := 0; i < domain.MaxPetsPerOwner; i++ {
		pets.pets[fmt.Sprintf("id-%d", i)] = domain.Pet{ID: fmt.Sprintf("id-%d", i), OwnerID: ownerA}
	}
	if _, err := svc.Create(ctx, ownerA, sampleDetails()); !errors.Is(err, domain.ErrPetLimitReached) {
		t.Fatalf("expected limit, got %v", err)
	}
}

func TestPetServicePhotoLifecycle(t *testing.T) {
	svc, pets, photos := newTestPetService()
	ctx := context.Background()
	pet, err := svc.Create(ctx, ownerA, sampleDetails())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetPhoto(ctx, ownerA, pet.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("no photo yet: %v", err)
	}
	if _, err := svc.PutPhoto(ctx, ownerA, pet.ID, []byte("GIF89a......")); !errors.Is(err, domain.ErrInvalidPetPhoto) {
		t.Fatalf("gif must be rejected: %v", err)
	}
	if _, err := svc.PutPhoto(ctx, ownerB, pet.ID, pngBytes); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other owner upload: %v", err)
	}
	withPhoto, err := svc.PutPhoto(ctx, ownerA, pet.ID, pngBytes)
	if err != nil || !withPhoto.HasPhoto {
		t.Fatalf("put photo: %+v %v", withPhoto, err)
	}
	photo, err := svc.GetPhoto(ctx, ownerA, pet.ID)
	if err != nil || photo.ContentType != "image/png" {
		t.Fatalf("photo=%+v err=%v", photo, err)
	}
	cleared, err := svc.DeletePhoto(ctx, ownerA, pet.ID)
	if err != nil || cleared.HasPhoto || len(photos.photos) != 0 {
		t.Fatalf("delete photo: %+v %v", cleared, err)
	}

	if _, err := svc.PutPhoto(ctx, ownerA, pet.ID, pngBytes); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, ownerA, pet.ID); err != nil {
		t.Fatal(err)
	}
	if len(photos.photos) != 0 || len(pets.pets) != 0 {
		t.Fatalf("delete must remove pet and photo: pets=%d photos=%d", len(pets.pets), len(photos.photos))
	}
}
