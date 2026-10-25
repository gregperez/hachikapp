package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
)

const testVisitor = "cccccccc-0000-4000-8000-000000000003"

type memScans struct {
	scans      map[string]domain.TagScan
	messages   []domain.TagMessage
	seq        int
	revealed   int
	lastDedupe time.Time
	drafts     []*domain.NotificationDraft
}

func newMemScans() *memScans { return &memScans{scans: map[string]domain.TagScan{}} }

func (m *memScans) RecordScan(_ context.Context, scan domain.TagScan, dedupeSince time.Time, draft *domain.NotificationDraft) (domain.TagScan, bool, error) {
	m.lastDedupe = dedupeSince
	for id, existing := range m.scans {
		if existing.TagID == scan.TagID && existing.VisitorHash == scan.VisitorHash && !existing.ScannedAt.Before(dedupeSince) {
			existing.ScanTokenHash = scan.ScanTokenHash
			m.scans[id] = existing
			return existing, true, nil
		}
	}
	m.seq++
	scan.ID = fmt.Sprintf("22222222-0000-4000-8000-%012d", m.seq)
	m.scans[scan.ID] = scan
	m.drafts = append(m.drafts, draft)
	return scan, false, nil
}

func (m *memScans) GetScan(_ context.Context, id string) (domain.TagScan, error) {
	scan, ok := m.scans[id]
	if !ok {
		return domain.TagScan{}, domain.ErrNotFound
	}
	return scan, nil
}

func (m *memScans) AddMessage(_ context.Context, msg domain.TagMessage, draft *domain.NotificationDraft) (domain.TagMessage, error) {
	m.drafts = append(m.drafts, draft)
	m.messages = append(m.messages, msg)
	return msg, nil
}

func (m *memScans) MarkPhoneRevealed(_ context.Context, id string, at time.Time, draft *domain.NotificationDraft) (bool, error) {
	scan, ok := m.scans[id]
	if !ok {
		return false, domain.ErrNotFound
	}
	if scan.PhoneRevealedAt != nil {
		return false, nil
	}
	scan.PhoneRevealedAt = &at
	m.drafts = append(m.drafts, draft)
	m.scans[id] = scan
	m.revealed++
	return true, nil
}

func (m *memScans) SetLocation(_ context.Context, id string, loc domain.ScanLocation, draft *domain.NotificationDraft) error {
	scan, ok := m.scans[id]
	if !ok {
		return domain.ErrNotFound
	}
	if scan.Location != nil {
		return domain.ErrLocationAlreadyShared
	}
	scan.Location = &loc
	m.scans[id] = scan
	m.drafts = append(m.drafts, draft)
	return nil
}

func (m *memScans) ListScansByPet(context.Context, string, string, authBaseDomain.PageRequest) (authBaseDomain.Page[domain.ScanWithMessages], error) {
	items := []domain.ScanWithMessages{}
	for _, s := range m.scans {
		items = append(items, domain.ScanWithMessages{TagScan: s})
	}
	return authBaseDomain.Page[domain.ScanWithMessages]{Items: items}, nil
}

type publicFixture struct {
	svc    *PublicTagService
	tags   *memTags
	scans  *memScans
	pets   *memPets
	photos *memPhotos
	tag    domain.PetTag
	pet    domain.Pet
}

var testPepper = []byte("test-pepper")

func newPublicFixture(t *testing.T, details domain.PetDetails) *publicFixture {
	t.Helper()
	ctx := context.Background()
	pets := newMemPets()
	if details.Name == "" {
		details.Name = "Luna"
	}
	if details.PublicVisibility == nil {
		details.PublicVisibility = domain.DefaultPetVisibility()
	}
	details.Owner = domain.PetOwnerContact{FullName: "Ana Pérez Soto", Address: "Calle 123", Comuna: "Providencia", Phone: "+56 9 1234 5678", PhoneAlt: "+56 2 2222 3333"}
	pet, err := pets.Create(ctx, ownerA, details)
	if err != nil {
		t.Fatal(err)
	}
	tags := &memTags{pets: pets}
	tagSvc := NewTagService(tags, pets, bytes.NewReader(bytes.Repeat([]byte{7, 9, 11, 13, 17}, 100)))
	tagSvc.now = func() time.Time { return time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC) }
	digital, err := tagSvc.CreateDigital(ctx, ownerA, pet.ID)
	if err != nil {
		t.Fatal(err)
	}
	photos := &memPhotos{photos: map[string]domain.PetPhoto{}}
	scans := newMemScans()
	svc := NewPublicTagService(tags, scans, pets, photos, testPepper, rand.Reader)
	svc.now = func() time.Time { return time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC) }
	return &publicFixture{svc: svc, tags: tags, scans: scans, pets: pets, photos: photos, tag: digital, pet: pet}
}

func withVisibility(field domain.PetPublicField, v domain.PetVisibility) domain.PetDetails {
	vis := domain.DefaultPetVisibility()
	vis[field] = v
	return domain.PetDetails{PublicVisibility: vis}
}

func (f *publicFixture) scan(t *testing.T) domain.ScanReceipt {
	t.Helper()
	r, err := f.svc.RecordScan(context.Background(), f.tag.Code, testVisitor, "203.0.113.9", "Mozilla/5.0 (iPhone)")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestViewUnknownOrMalformedCode(t *testing.T) {
	f := newPublicFixture(t, domain.PetDetails{})
	for _, code := range []string{"!!", "", "ZZZZZZZZZZ"} {
		if _, err := f.svc.View(context.Background(), code); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("code %q: %v", code, err)
		}
	}
}

func TestViewActiveHasProfileAndNormalizesCode(t *testing.T) {
	f := newPublicFixture(t, domain.PetDetails{})
	view, err := f.svc.View(context.Background(), strings.ToLower(f.tag.Code))
	if err != nil || view.State != domain.TagStatusActive || view.Profile == nil || view.Profile.Name != "Luna" {
		t.Fatalf("view=%+v err=%v", view, err)
	}
	if view.Profile.Lost {
		t.Fatal("lost must be false")
	}
}

func TestViewDisabledHasNoProfile(t *testing.T) {
	f := newPublicFixture(t, domain.PetDetails{})
	if _, err := f.tags.Disable(context.Background(), ownerA, f.pet.ID, f.tag.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	view, err := f.svc.View(context.Background(), f.tag.Code)
	if err != nil || view.State != domain.TagStatusDisabled || view.Profile != nil {
		t.Fatalf("view=%+v err=%v", view, err)
	}
}

func TestViewUnassignedHasNoProfile(t *testing.T) {
	f := newPublicFixture(t, domain.PetDetails{})
	f.tags.tags = append(f.tags.tags, domain.PetTag{ID: f.tags.nextID(), Code: "ABCDEFGH23", Status: domain.TagStatusUnassigned})
	view, err := f.svc.View(context.Background(), "abcdefgh23")
	if err != nil || view.State != domain.TagStatusUnassigned || view.Profile != nil {
		t.Fatalf("view=%+v err=%v", view, err)
	}
}

func TestViewUnassignedAfterPetDeletion(t *testing.T) {
	f := newPublicFixture(t, domain.PetDetails{})
	if err := f.pets.Delete(context.Background(), ownerA, f.pet.ID); err != nil {
		t.Fatal(err)
	}
	view, err := f.svc.View(context.Background(), f.tag.Code)
	if err != nil || view.State != domain.TagStatusUnassigned || view.Profile != nil {
		t.Fatalf("view=%+v err=%v", view, err)
	}
}

func TestPhotoHiddenWhenPhotoNotVisible(t *testing.T) {
	f := newPublicFixture(t, withVisibility(domain.PetPublicPhoto, domain.PetVisibilityHidden))
	at := time.Now()
	if _, err := f.pets.SetPhotoUpdatedAt(context.Background(), ownerA, f.pet.ID, &at); err != nil {
		t.Fatal(err)
	}
	f.photos.photos[f.pet.ID] = domain.PetPhoto{ContentType: "image/png", Data: []byte{1}}
	if _, err := f.svc.Photo(context.Background(), f.tag.Code); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestPhotoServedWhenVisible(t *testing.T) {
	f := newPublicFixture(t, withVisibility(domain.PetPublicPhoto, domain.PetVisibilityAlways))
	at := time.Now()
	if _, err := f.pets.SetPhotoUpdatedAt(context.Background(), ownerA, f.pet.ID, &at); err != nil {
		t.Fatal(err)
	}
	f.photos.photos[f.pet.ID] = domain.PetPhoto{ContentType: "image/png", Data: []byte{1}}
	photo, err := f.svc.Photo(context.Background(), f.tag.Code)
	if err != nil || photo.ContentType != "image/png" {
		t.Fatalf("photo=%+v err=%v", photo, err)
	}
	// Pet without a stored photo flag, and disabled tags, are 404.
	if _, err := f.svc.Photo(context.Background(), "nope!"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("bad code: %v", err)
	}
	if _, err := f.tags.Disable(context.Background(), ownerA, f.pet.ID, f.tag.ID, at); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Photo(context.Background(), f.tag.Code); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("disabled: %v", err)
	}
}

func TestPhotoNotFoundWhenPetHasNoPhoto(t *testing.T) {
	f := newPublicFixture(t, withVisibility(domain.PetPublicPhoto, domain.PetVisibilityAlways))
	if _, err := f.svc.Photo(context.Background(), f.tag.Code); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestRecordScanOnlyForActiveTags(t *testing.T) {
	f := newPublicFixture(t, domain.PetDetails{})
	f.tags.tags = append(f.tags.tags, domain.PetTag{ID: f.tags.nextID(), Code: "ABCDEFGH23", Status: domain.TagStatusUnassigned})
	if _, err := f.svc.RecordScan(context.Background(), "ABCDEFGH23", testVisitor, "1.2.3.4", "ua"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unassigned: %v", err)
	}
	if _, err := f.svc.RecordScan(context.Background(), "??", testVisitor, "1.2.3.4", "ua"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("malformed: %v", err)
	}
	if len(f.scans.scans) != 0 {
		t.Fatal("no scan may be stored")
	}
}

func TestRecordScanInvalidVisitorID(t *testing.T) {
	f := newPublicFixture(t, domain.PetDetails{})
	_, err := f.svc.RecordScan(context.Background(), f.tag.Code, "not-a-uuid", "1.2.3.4", "ua")
	var verr *domain.PetValidationError
	if !errors.Is(err, domain.ErrInvalidPet) || !errors.As(err, &verr) || len(verr.Issues) != 1 || verr.Issues[0].Field != "visitorId" || verr.Issues[0].Code != "invalid" {
		t.Fatalf("err=%v", err)
	}
}

func TestRecordScanStoresHashesOnly(t *testing.T) {
	f := newPublicFixture(t, domain.PetDetails{})
	upper := strings.ToUpper(testVisitor)
	receipt, err := f.svc.RecordScan(context.Background(), f.tag.Code, " "+upper+" ", "203.0.113.9", "Mozilla/5.0 (iPhone)")
	if err != nil || receipt.ScanID == "" || receipt.Token == "" {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	stored := f.scans.scans[receipt.ScanID]
	if stored.VisitorHash != domain.KeyedHash(testPepper, f.tag.ID+"|"+testVisitor) {
		t.Fatalf("visitor hash=%s", stored.VisitorHash)
	}
	if stored.ScanTokenHash != domain.KeyedHash(testPepper, receipt.Token) || stored.ScanTokenHash == receipt.Token {
		t.Fatalf("token hash=%s", stored.ScanTokenHash)
	}
	if stored.IPPrefix != "203.0.113.0/24" || stored.UserAgentFamily != "ios" || stored.PetID != f.pet.ID || stored.TagID != f.tag.ID {
		t.Fatalf("stored=%+v", stored)
	}
	if !stored.ScannedAt.Equal(f.svc.now()) || !f.scans.lastDedupe.Equal(f.svc.now().Add(-domain.ScanDedupeWindow)) {
		t.Fatalf("times: %v %v", stored.ScannedAt, f.scans.lastDedupe)
	}
	// Same visitor again: same scan, fresh token that replaces the old one.
	again, err := f.svc.RecordScan(context.Background(), f.tag.Code, testVisitor, "203.0.113.9", "ua")
	if err != nil || again.ScanID != receipt.ScanID || again.Token == receipt.Token {
		t.Fatalf("again=%+v err=%v", again, err)
	}
	if err := f.svc.SendMessage(context.Background(), receipt.ScanID, receipt.Token, domain.FinderMessageInput{Body: "hola"}); !errors.Is(err, domain.ErrInvalidScanToken) {
		t.Fatalf("old token should be invalid: %v", err)
	}
}

func TestSendMessageRequiresMatchingToken(t *testing.T) {
	f := newPublicFixture(t, domain.PetDetails{})
	r := f.scan(t)
	in := domain.FinderMessageInput{Body: "Lo encontré"}
	for _, token := range []string{"", "wrong", r.Token + "x"} {
		if err := f.svc.SendMessage(context.Background(), r.ScanID, token, in); !errors.Is(err, domain.ErrInvalidScanToken) {
			t.Fatalf("token %q: %v", token, err)
		}
	}
	if err := f.svc.SendMessage(context.Background(), "not-a-uuid", r.Token, in); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("bad scan id: %v", err)
	}
	if err := f.svc.SendMessage(context.Background(), "22222222-0000-4000-8000-0000000000ff", r.Token, in); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown scan: %v", err)
	}
	if len(f.scans.messages) != 0 {
		t.Fatal("no message may be stored")
	}
}

func TestSendMessageStoresTrimmedMessage(t *testing.T) {
	f := newPublicFixture(t, withVisibility(domain.PetPublicContactButton, domain.PetVisibilityHidden))
	r := f.scan(t)
	err := f.svc.SendMessage(context.Background(), r.ScanID, r.Token, domain.FinderMessageInput{Body: "  Lo tengo en la plaza \r\n", FinderName: "  Pedro   Gómez ", FinderPhone: "+56 9 8765 4321"})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.scans.messages) != 1 {
		t.Fatalf("messages=%d", len(f.scans.messages))
	}
	m := f.scans.messages[0]
	if m.Body != "Lo tengo en la plaza" || m.FinderName != "Pedro Gómez" || m.ScanID != r.ScanID || m.TagID != f.tag.ID || m.PetID != f.pet.ID {
		t.Fatalf("message=%+v", m)
	}
}

func TestSendMessageValidates(t *testing.T) {
	f := newPublicFixture(t, domain.PetDetails{})
	r := f.scan(t)
	err := f.svc.SendMessage(context.Background(), r.ScanID, r.Token, domain.FinderMessageInput{Body: "   "})
	if !errors.Is(err, domain.ErrInvalidPet) {
		t.Fatalf("err=%v", err)
	}
	if len(f.scans.messages) != 0 {
		t.Fatal("stored")
	}
}

func TestRevealContactHonoursContactButton(t *testing.T) {
	hidden := newPublicFixture(t, withVisibility(domain.PetPublicContactButton, domain.PetVisibilityHidden))
	r := hidden.scan(t)
	if _, err := hidden.svc.RevealContact(context.Background(), r.ScanID, r.Token); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("hidden: %v", err)
	}
	if hidden.scans.revealed != 0 {
		t.Fatal("must not mark revealed")
	}

	f := newPublicFixture(t, withVisibility(domain.PetPublicContactButton, domain.PetVisibilityAlways))
	r = f.scan(t)
	c, err := f.svc.RevealContact(context.Background(), r.ScanID, r.Token)
	if err != nil || c.Phone != "+56 9 1234 5678" || c.WhatsAppURL != "https://wa.me/56912345678" {
		t.Fatalf("contact=%+v err=%v", c, err)
	}
	if f.scans.revealed != 1 {
		t.Fatalf("revealed=%d", f.scans.revealed)
	}
	// Repeating is allowed and still succeeds.
	if _, err := f.svc.RevealContact(context.Background(), r.ScanID, r.Token); err != nil {
		t.Fatal(err)
	}
}

func TestRevealContactLostOnlyIsHidden(t *testing.T) {
	f := newPublicFixture(t, withVisibility(domain.PetPublicContactButton, domain.PetVisibilityLostOnly))
	r := f.scan(t)
	if _, err := f.svc.RevealContact(context.Background(), r.ScanID, r.Token); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestRevealContactNeverReturnsPhoneAlt(t *testing.T) {
	f := newPublicFixture(t, withVisibility(domain.PetPublicContactButton, domain.PetVisibilityAlways))
	r := f.scan(t)
	c, err := f.svc.RevealContact(context.Background(), r.ScanID, r.Token)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(c.Phone, "2222") || strings.Contains(c.WhatsAppURL, "2222") {
		t.Fatalf("phoneAlt leaked: %+v", c)
	}
}

func TestRevealContactTokenMismatchAndEmptyPhone(t *testing.T) {
	f := newPublicFixture(t, withVisibility(domain.PetPublicContactButton, domain.PetVisibilityAlways))
	r := f.scan(t)
	if _, err := f.svc.RevealContact(context.Background(), r.ScanID, "bad"); !errors.Is(err, domain.ErrInvalidScanToken) {
		t.Fatalf("mismatch: %v", err)
	}
	if _, err := f.svc.RevealContact(context.Background(), r.ScanID, ""); !errors.Is(err, domain.ErrInvalidScanToken) {
		t.Fatalf("empty: %v", err)
	}
	pet := f.pets.pets[f.pet.ID]
	pet.Owner.Phone = ""
	f.pets.pets[f.pet.ID] = pet
	if _, err := f.svc.RevealContact(context.Background(), r.ScanID, r.Token); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("no phone: %v", err)
	}
}

func TestRevealContactAfterTagDisabled(t *testing.T) {
	f := newPublicFixture(t, withVisibility(domain.PetPublicContactButton, domain.PetVisibilityAlways))
	r := f.scan(t)
	if _, err := f.tags.Disable(context.Background(), ownerA, f.pet.ID, f.tag.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.RevealContact(context.Background(), r.ScanID, r.Token); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("reveal: %v", err)
	}
	if err := f.svc.SendMessage(context.Background(), r.ScanID, r.Token, domain.FinderMessageInput{Body: "hola"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("message: %v", err)
	}
}

func TestRevealContactAfterPetDeleted(t *testing.T) {
	f := newPublicFixture(t, withVisibility(domain.PetPublicContactButton, domain.PetVisibilityAlways))
	r := f.scan(t)
	if err := f.pets.Delete(context.Background(), ownerA, f.pet.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.RevealContact(context.Background(), r.ScanID, r.Token); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestWhatsAppURL(t *testing.T) {
	cases := map[string]string{
		"+56 9 1234 5678":    "https://wa.me/56912345678",
		"9 1234 5678":        "https://wa.me/56912345678",
		"(+54) 11-5555-1234": "https://wa.me/541155551234",
	}
	for phone, want := range cases {
		if got := whatsAppURL(phone); got != want {
			t.Errorf("%q: got %q want %q", phone, got, want)
		}
	}
}

func TestListPetScansIsOwnerScoped(t *testing.T) {
	f := newPublicFixture(t, domain.PetDetails{})
	f.scan(t)
	page, err := f.svc.ListPetScans(context.Background(), ownerA, f.pet.ID, authBaseDomain.PageRequest{})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if _, err := f.svc.ListPetScans(context.Background(), ownerB, f.pet.ID, authBaseDomain.PageRequest{}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign: %v", err)
	}
	if _, err := f.svc.ListPetScans(context.Background(), ownerA, "bad", authBaseDomain.PageRequest{}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("bad id: %v", err)
	}
}

func lastDraft(t *testing.T, m *memScans) *domain.NotificationDraft {
	t.Helper()
	if len(m.drafts) == 0 {
		t.Fatal("no draft recorded")
	}
	return m.drafts[len(m.drafts)-1]
}

func TestRecordScanBuildsScanDraft(t *testing.T) {
	f := newPublicFixture(t, domain.PetDetails{})
	if _, err := f.svc.RecordScan(context.Background(), f.tag.Code, testVisitor, "203.0.113.9", "Mozilla/5.0 (iPhone)"); err != nil {
		t.Fatal(err)
	}
	d := lastDraft(t, f.scans)
	if d == nil {
		t.Fatal("nil draft")
	}
	if d.UserID != ownerA || d.PetID != f.pet.ID || d.TagID != f.tag.ID || d.Kind != domain.NotificationTagScanned || !d.Email {
		t.Fatalf("draft=%+v", d)
	}
	want := domain.NotificationPayload{PetName: "Luna", TagCode: f.tag.Code, ScannedAt: f.svc.now(), Device: "ios"}
	if d.Payload != want {
		t.Fatalf("payload=%+v want %+v", d.Payload, want)
	}
}

func TestSendMessageBuildsMessageDraft(t *testing.T) {
	f := newPublicFixture(t, domain.PetDetails{})
	r, _ := f.svc.RecordScan(context.Background(), f.tag.Code, testVisitor, "1.2.3.4", "Android")
	err := f.svc.SendMessage(context.Background(), r.ScanID, r.Token, domain.FinderMessageInput{Body: "  La encontré  ", FinderName: " Ana ", FinderPhone: "+56 9 1111 2222"})
	if err != nil {
		t.Fatal(err)
	}
	d := lastDraft(t, f.scans)
	if d == nil || d.Kind != domain.NotificationFinderMessage || d.UserID != ownerA || !d.Email {
		t.Fatalf("draft=%+v", d)
	}
	m := d.Payload.Message
	if m == nil || m.Body != "La encontré" || m.FinderName != "Ana" || m.FinderPhone != f.scans.messages[0].FinderPhone {
		t.Fatalf("message=%+v", m)
	}
	if d.Payload.PetName != "Luna" || d.Payload.Device != "android" || d.Payload.TagCode != f.tag.Code {
		t.Fatalf("payload=%+v", d.Payload)
	}
}

func TestRevealContactBuildsPhoneDraft(t *testing.T) {
	f := newPublicFixture(t, domain.PetDetails{})
	r, _ := f.svc.RecordScan(context.Background(), f.tag.Code, testVisitor, "1.2.3.4", "Mozilla/5.0 (iPhone)")
	if _, err := f.svc.RevealContact(context.Background(), r.ScanID, r.Token); err != nil {
		t.Fatal(err)
	}
	d := lastDraft(t, f.scans)
	if d == nil || d.Kind != domain.NotificationPhoneRevealed || !d.Payload.PhoneRevealed || d.UserID != ownerA || !d.Email {
		t.Fatalf("draft=%+v", d)
	}
}

func TestEmailFlagOff(t *testing.T) {
	f := newPublicFixture(t, domain.PetDetails{})
	f.svc.WithEmailNotifications(false)
	r, _ := f.svc.RecordScan(context.Background(), f.tag.Code, testVisitor, "1.2.3.4", "ua")
	_ = f.svc.SendMessage(context.Background(), r.ScanID, r.Token, domain.FinderMessageInput{Body: "hola hola hola"})
	_, _ = f.svc.RevealContact(context.Background(), r.ScanID, r.Token)
	if len(f.scans.drafts) != 3 {
		t.Fatalf("drafts=%d", len(f.scans.drafts))
	}
	for i, d := range f.scans.drafts {
		if d == nil || d.Email {
			t.Fatalf("draft %d=%+v", i, d)
		}
	}
}

func TestShareLocationValidatesAndStores(t *testing.T) {
	f := newPublicFixture(t, domain.PetDetails{})
	r := f.scan(t)
	ctx := context.Background()

	for _, bad := range []struct {
		lat, lng float64
		acc      int
		field    string
	}{{91, 0, 10, "lat"}, {0, -181, 10, "lng"}, {0, 0, -1, "accuracy"}, {0, 0, 100001, "accuracy"}} {
		err := f.svc.ShareLocation(ctx, r.ScanID, r.Token, bad.lat, bad.lng, bad.acc)
		var ve *domain.PetValidationError
		if !errors.As(err, &ve) || ve.Issues[0].Field != bad.field {
			t.Fatalf("%+v: %v", bad, err)
		}
	}
	if f.scans.scans[r.ScanID].Location != nil {
		t.Fatal("invalid input must not be stored")
	}

	// 0,0 and the range edges are valid values.
	if err := f.svc.ShareLocation(ctx, r.ScanID, r.Token, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	loc := f.scans.scans[r.ScanID].Location
	if loc == nil || loc.Lat != 0 || loc.Lng != 0 || loc.AccuracyM != 0 || !loc.At.Equal(f.svc.now()) {
		t.Fatalf("loc=%+v", loc)
	}
	d := lastDraft(t, f.scans)
	if d == nil || d.Kind != domain.NotificationLocationShared || d.UserID != ownerA || !d.Email || d.Payload.PetName != "Luna" {
		t.Fatalf("draft=%+v", d)
	}
	if d.Payload.Location == nil || *d.Payload.Location != *loc {
		t.Fatalf("payload location=%+v", d.Payload.Location)
	}
}

func TestShareLocationTwiceConflict(t *testing.T) {
	f := newPublicFixture(t, domain.PetDetails{})
	r := f.scan(t)
	ctx := context.Background()
	if err := f.svc.ShareLocation(ctx, r.ScanID, r.Token, -33.4, -70.6, 25); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ShareLocation(ctx, r.ScanID, r.Token, -33.5, -70.7, 30); !errors.Is(err, domain.ErrLocationAlreadyShared) {
		t.Fatalf("err=%v", err)
	}
	if got := f.scans.scans[r.ScanID].Location; got.Lat != -33.4 {
		t.Fatalf("first location must win: %+v", got)
	}
}

func TestShareLocationBadToken404(t *testing.T) {
	f := newPublicFixture(t, domain.PetDetails{})
	r := f.scan(t)
	ctx := context.Background()
	for _, token := range []string{"", "wrong"} {
		if err := f.svc.ShareLocation(ctx, r.ScanID, token, 1, 1, 1); !errors.Is(err, domain.ErrInvalidScanToken) {
			t.Fatalf("token %q: %v", token, err)
		}
	}
	// Auth comes before validation: a bad token with bad input is still the token error.
	if err := f.svc.ShareLocation(ctx, r.ScanID, "wrong", 999, 1, 1); !errors.Is(err, domain.ErrInvalidScanToken) {
		t.Fatalf("err=%v", err)
	}
	if err := f.svc.ShareLocation(ctx, "not-a-uuid", r.Token, 1, 1, 1); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
	if _, err := f.tags.Disable(ctx, ownerA, f.pet.ID, f.tag.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ShareLocation(ctx, r.ScanID, r.Token, 1, 1, 1); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("disabled tag: %v", err)
	}
	if f.scans.scans[r.ScanID].Location != nil {
		t.Fatal("nothing may be stored")
	}
}
