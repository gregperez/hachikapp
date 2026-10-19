package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"testing"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

type memTags struct {
	pets         *memPets
	tags         []domain.PetTag
	batches      []domain.TagBatch
	dupFailures  int
	batchCalls   int
	digitalCalls int
	seq          int
}

func (m *memTags) nextID() string {
	m.seq++
	return fmt.Sprintf("11111111-0000-4000-8000-%012d", m.seq)
}

func (m *memTags) CreateBatch(_ context.Context, batch domain.TagBatch, codes []string) (domain.TagBatch, error) {
	m.batchCalls++
	if m.dupFailures > 0 {
		m.dupFailures--
		return domain.TagBatch{}, domain.ErrDuplicateTagCode
	}
	batch.ID = m.nextID()
	batch.Quantity = len(codes)
	m.batches = append(m.batches, batch)
	for _, code := range codes {
		m.tags = append(m.tags, domain.PetTag{ID: m.nextID(), Code: code, Kind: domain.TagKindPhysical, Status: domain.TagStatusUnassigned, BatchID: batch.ID})
	}
	return batch, nil
}

func (m *memTags) ListBatches(context.Context, authBaseDomain.PageRequest) (authBaseDomain.Page[domain.TagBatch], error) {
	return authBaseDomain.Page[domain.TagBatch]{Items: m.batches}, nil
}

func (m *memTags) BatchTags(_ context.Context, batchID string) ([]domain.PetTag, error) {
	out := []domain.PetTag{}
	for _, tag := range m.tags {
		if tag.BatchID == batchID {
			out = append(out, tag)
		}
	}
	return out, nil
}

func (m *memTags) GetByCode(_ context.Context, code string) (domain.PetTag, error) {
	for _, tag := range m.tags {
		if tag.Code == code {
			return tag, nil
		}
	}
	return domain.PetTag{}, domain.ErrNotFound
}

func (m *memTags) ListByPet(_ context.Context, ownerID, petID string) ([]domain.PetTag, error) {
	out := []domain.PetTag{}
	for _, tag := range m.tags {
		if tag.PetID == petID && tag.OwnerID == ownerID {
			out = append(out, tag)
		}
	}
	return out, nil
}

// guard emulates the store's atomic ownership and limit checks.
func (m *memTags) guard(ownerID, petID string, maxActive int) error {
	pet, ok := m.pets.pets[petID]
	if !ok || pet.OwnerID != ownerID {
		return domain.ErrNotFound
	}
	active := 0
	for _, tag := range m.tags {
		if tag.PetID == petID && tag.Status == domain.TagStatusActive {
			active++
		}
	}
	if active >= maxActive {
		return domain.ErrTagLimitReached
	}
	return nil
}

func (m *memTags) Activate(_ context.Context, code, ownerID, petID string, maxActive int, at time.Time) (domain.PetTag, error) {
	if err := m.guard(ownerID, petID, maxActive); err != nil {
		return domain.PetTag{}, err
	}
	for i, tag := range m.tags {
		if tag.Code == code && tag.CheckActivatableBy(ownerID) == nil {
			tag.Status, tag.PetID, tag.OwnerID, tag.ActivatedAt = domain.TagStatusActive, petID, ownerID, &at
			m.tags[i] = tag
			return tag, nil
		}
	}
	return domain.PetTag{}, domain.ErrTagNotAssignable
}

func (m *memTags) CreateDigital(_ context.Context, ownerID, petID, code string, maxActive int, at time.Time) (domain.PetTag, error) {
	m.digitalCalls++
	if m.dupFailures > 0 {
		m.dupFailures--
		return domain.PetTag{}, domain.ErrDuplicateTagCode
	}
	if err := m.guard(ownerID, petID, maxActive); err != nil {
		return domain.PetTag{}, err
	}
	tag := domain.PetTag{ID: m.nextID(), Code: code, Kind: domain.TagKindDigital, Status: domain.TagStatusActive, PetID: petID, OwnerID: ownerID, ActivatedAt: &at}
	m.tags = append(m.tags, tag)
	return tag, nil
}

func (m *memTags) Disable(_ context.Context, ownerID, petID, tagID string, at time.Time) (domain.PetTag, error) {
	for i, tag := range m.tags {
		if tag.ID == tagID && tag.PetID == petID && tag.OwnerID == ownerID && tag.Status == domain.TagStatusActive {
			tag.Status, tag.DisabledAt = domain.TagStatusDisabled, &at
			m.tags[i] = tag
			return tag, nil
		}
	}
	return domain.PetTag{}, domain.ErrNotFound
}

func newTagFixture(t *testing.T) (*TagService, *memTags, string) {
	t.Helper()
	pets := newMemPets()
	pet, err := pets.Create(context.Background(), ownerA, domain.PetDetails{Name: "Luna"})
	if err != nil {
		t.Fatal(err)
	}
	tags := &memTags{pets: pets}
	svc := NewTagService(tags, pets, rand.Reader)
	svc.now = func() time.Time { return time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC) }
	return svc, tags, pet.ID
}

func seedBatch(t *testing.T, svc *TagService, n int) []domain.PetTag {
	t.Helper()
	batch, err := svc.CreateBatch(context.Background(), ownerA, "Lote", n)
	if err != nil {
		t.Fatal(err)
	}
	tags, err := svc.BatchTags(context.Background(), batch.ID)
	if err != nil || len(tags) != n {
		t.Fatalf("tags=%d err=%v", len(tags), err)
	}
	return tags
}

func TestTagServiceActivateRequiresOwnedPet(t *testing.T) {
	svc, _, petID := newTagFixture(t)
	tags := seedBatch(t, svc, 1)
	if _, err := svc.Activate(context.Background(), ownerB, tags[0].Code, petID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

func TestTagServiceActivateNormalizesCode(t *testing.T) {
	svc, tags, petID := newTagFixture(t)
	tags.tags = append(tags.tags, domain.PetTag{ID: tags.nextID(), Code: "K7M2Q9XR4P", Kind: domain.TagKindPhysical, Status: domain.TagStatusUnassigned})
	got, err := svc.Activate(context.Background(), ownerA, "k7m2-q9xr4p", petID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Code != "K7M2Q9XR4P" || got.Status != domain.TagStatusActive || got.PetID != petID {
		t.Fatalf("got %+v", got)
	}
}

func TestTagServiceActivateRejectsBadInput(t *testing.T) {
	svc, tags, petID := newTagFixture(t)
	if _, err := svc.Activate(context.Background(), ownerA, "not a code!", petID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("bad code: %v", err)
	}
	if _, err := svc.Activate(context.Background(), ownerA, "K7M2Q9XR4P", "not-a-uuid"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("bad pet id: %v", err)
	}
	if len(tags.tags) != 0 {
		t.Fatal("store must not be touched")
	}
}

func TestTagServiceEnforcesFiveActiveTags(t *testing.T) {
	svc, _, petID := newTagFixture(t)
	ctx := context.Background()
	batch := seedBatch(t, svc, 3)
	for i := 0; i < 3; i++ {
		if _, err := svc.Activate(ctx, ownerA, batch[i].Code, petID); err != nil {
			t.Fatalf("activation %d: %v", i, err)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := svc.CreateDigital(ctx, ownerA, petID); err != nil {
			t.Fatalf("digital %d: %v", i, err)
		}
	}
	extra := seedBatch(t, svc, 1)
	if _, err := svc.Activate(ctx, ownerA, extra[0].Code, petID); !errors.Is(err, domain.ErrTagLimitReached) {
		t.Fatalf("sixth activation: %v", err)
	}
	if _, err := svc.CreateDigital(ctx, ownerA, petID); !errors.Is(err, domain.ErrTagLimitReached) {
		t.Fatalf("sixth digital: %v", err)
	}
}

func TestTagServiceRetriesCodeCollision(t *testing.T) {
	svc, tags, petID := newTagFixture(t)
	tags.dupFailures = 1
	batch, err := svc.CreateBatch(context.Background(), ownerA, "  Lote  ", 3)
	if err != nil {
		t.Fatal(err)
	}
	if tags.batchCalls != 2 || batch.Label != "Lote" || batch.CreatedBy != ownerA || len(tags.tags) != 3 {
		t.Fatalf("calls=%d batch=%+v tags=%d", tags.batchCalls, batch, len(tags.tags))
	}
	tags.dupFailures = 1
	if _, err := svc.CreateDigital(context.Background(), ownerA, petID); err != nil || tags.digitalCalls != 2 {
		t.Fatalf("digital calls=%d err=%v", tags.digitalCalls, err)
	}
	tags.dupFailures = 3
	tags.batchCalls = 0
	if _, err := svc.CreateBatch(context.Background(), ownerA, "Otro", 1); !errors.Is(err, domain.ErrDuplicateTagCode) || tags.batchCalls != 3 {
		t.Fatalf("calls=%d err=%v", tags.batchCalls, err)
	}
}

func TestTagServiceCreateBatchGeneratesUniqueCodes(t *testing.T) {
	// A reader that repeats the same bytes would repeat codes; the service must dedupe within the batch.
	pets := newMemPets()
	tags := &memTags{pets: pets}
	pattern := bytes.Repeat([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 1)
	var stream []byte
	stream = append(stream, pattern...)
	stream = append(stream, pattern...) // duplicate code
	stream = append(stream, bytes.Repeat([]byte{11, 12, 13, 14, 15, 16, 17, 18, 19, 20}, 1)...)
	svc := NewTagService(tags, pets, bytes.NewReader(stream))
	if _, err := svc.CreateBatch(context.Background(), ownerA, "Lote", 2); err != nil {
		t.Fatal(err)
	}
	if len(tags.tags) != 2 || tags.tags[0].Code == tags.tags[1].Code {
		t.Fatalf("tags=%+v", tags.tags)
	}
}

func TestTagServiceCreateBatchValidates(t *testing.T) {
	svc, tags, _ := newTagFixture(t)
	_, err := svc.CreateBatch(context.Background(), ownerA, "Lote", 1001)
	var validation *domain.PetValidationError
	if !errors.Is(err, domain.ErrInvalidPet) || !errors.As(err, &validation) {
		t.Fatalf("got %v", err)
	}
	if tags.batchCalls != 0 {
		t.Fatal("port must not be called")
	}
}

func TestTagServiceDisableAndList(t *testing.T) {
	svc, _, petID := newTagFixture(t)
	ctx := context.Background()
	digital, err := svc.CreateDigital(ctx, ownerA, petID)
	if err != nil || digital.Kind != domain.TagKindDigital {
		t.Fatalf("digital=%+v err=%v", digital, err)
	}
	list, err := svc.ListPetTags(ctx, ownerA, petID)
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}
	if _, err := svc.ListPetTags(ctx, ownerB, petID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign list: %v", err)
	}
	if _, err := svc.Disable(ctx, ownerA, petID, "bad"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("bad tag id: %v", err)
	}
	disabled, err := svc.Disable(ctx, ownerA, petID, digital.ID)
	if err != nil || disabled.Status != domain.TagStatusDisabled {
		t.Fatalf("disabled=%+v err=%v", disabled, err)
	}
	if _, err := svc.BatchTags(ctx, "bad"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("bad batch id: %v", err)
	}
	if _, err := svc.ListBatches(ctx, authBaseDomain.PageRequest{Limit: 10}); err != nil {
		t.Fatal(err)
	}
}
