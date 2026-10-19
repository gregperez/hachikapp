package services

import (
	"context"
	"crypto/subtle"
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

// PublicTagService implements the public (no login) tag use cases. It is
// privacy critical: the owner's phone is only returned by RevealContact, and
// addresses, chip numbers and surnames are never returned.
type PublicTagService struct {
	tags   ports.TagPort
	scans  ports.TagScanPort
	pets   ports.PetPort
	photos ports.PetPhotoPort
	pepper []byte
	random io.Reader
	now    func() time.Time
}

// NewPublicTagService builds the service. pepper keys the visitor and scan
// token hashes and must not be empty: it panics otherwise (the configuration
// layer should validate it before getting here).
func NewPublicTagService(tags ports.TagPort, scans ports.TagScanPort, pets ports.PetPort, photos ports.PetPhotoPort, pepper []byte, random io.Reader) *PublicTagService {
	if len(pepper) == 0 {
		panic("public tag service: pepper must not be empty")
	}
	return &PublicTagService{
		tags:   tags,
		scans:  scans,
		pets:   pets,
		photos: photos,
		pepper: pepper,
		random: random,
		now:    func() time.Time { return time.Now().UTC() },
	}
}

var _ core.PublicTagUseCase = (*PublicTagService)(nil)

func (s *PublicTagService) View(ctx context.Context, code string) (domain.PublicTagView, error) {
	tag, err := s.tagByCode(ctx, code)
	if err != nil {
		return domain.PublicTagView{}, err
	}
	switch tag.Status {
	case domain.TagStatusDisabled:
		return domain.PublicTagView{State: domain.TagStatusDisabled}, nil
	case domain.TagStatusActive:
	default:
		return domain.PublicTagView{State: domain.TagStatusUnassigned}, nil
	}
	pet, err := s.pets.GetForOwner(ctx, tag.OwnerID, tag.PetID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.PublicTagView{State: domain.TagStatusUnassigned}, nil
	}
	if err != nil {
		return domain.PublicTagView{}, err
	}
	profile := domain.PublicProfileOf(pet, false, s.now())
	return domain.PublicTagView{State: domain.TagStatusActive, Profile: &profile}, nil
}

func (s *PublicTagService) Photo(ctx context.Context, code string) (domain.PetPhoto, error) {
	_, pet, err := s.activeTagAndPet(ctx, code)
	if err != nil {
		return domain.PetPhoto{}, err
	}
	if !domain.PublicProfileOf(pet, false, s.now()).HasPhoto {
		return domain.PetPhoto{}, domain.ErrNotFound
	}
	return s.photos.Get(ctx, pet.ID)
}

func (s *PublicTagService) RecordScan(ctx context.Context, code, visitorID, ip, userAgent string) (domain.ScanReceipt, error) {
	tag, pet, err := s.activeTagAndPet(ctx, code)
	if err != nil {
		return domain.ScanReceipt{}, err
	}
	visitor := strings.TrimSpace(visitorID)
	if !isPetID(visitor) {
		return domain.ScanReceipt{}, &domain.PetValidationError{Issues: []domain.PetFieldIssue{{Field: "visitorId", Code: "invalid"}}}
	}
	token, err := domain.NewScanToken(s.random)
	if err != nil {
		return domain.ScanReceipt{}, fmt.Errorf("generate scan token: %w", err)
	}
	now := s.now()
	scan, _, err := s.scans.RecordScan(ctx, domain.TagScan{
		TagID:           tag.ID,
		PetID:           pet.ID,
		ScannedAt:       now,
		VisitorHash:     domain.KeyedHash(s.pepper, tag.ID+"|"+strings.ToLower(visitor)),
		ScanTokenHash:   domain.KeyedHash(s.pepper, token),
		IPPrefix:        domain.IPPrefix(ip),
		UserAgentFamily: domain.UserAgentFamily(userAgent),
	}, now.Add(-domain.ScanDedupeWindow))
	if err != nil {
		return domain.ScanReceipt{}, err
	}
	return domain.ScanReceipt{ScanID: scan.ID, Token: token}, nil
}

func (s *PublicTagService) SendMessage(ctx context.Context, scanID, token string, in domain.FinderMessageInput) error {
	scan, _, _, err := s.authorizedScan(ctx, scanID, token)
	if err != nil {
		return err
	}
	in = domain.NormalizeFinderMessage(in)
	if err := domain.ValidateFinderMessage(in); err != nil {
		return err
	}
	_, err = s.scans.AddMessage(ctx, domain.TagMessage{
		ScanID:      scan.ID,
		TagID:       scan.TagID,
		PetID:       scan.PetID,
		Body:        in.Body,
		FinderName:  in.FinderName,
		FinderPhone: in.FinderPhone,
	})
	return err
}

func (s *PublicTagService) RevealContact(ctx context.Context, scanID, token string) (domain.PublicContact, error) {
	scan, _, pet, err := s.authorizedScan(ctx, scanID, token)
	if err != nil {
		return domain.PublicContact{}, err
	}
	phone := pet.Owner.Phone
	if !domain.PublicProfileOf(pet, false, s.now()).CanContact || strings.TrimSpace(phone) == "" {
		return domain.PublicContact{}, domain.ErrNotFound
	}
	// Part 2 uses the result to notify the owner on the first reveal only.
	if _, err := s.scans.MarkPhoneRevealed(ctx, scan.ID, s.now()); err != nil {
		return domain.PublicContact{}, err
	}
	return domain.PublicContact{Phone: phone, WhatsAppURL: whatsAppURL(phone)}, nil
}

func (s *PublicTagService) ListPetScans(ctx context.Context, ownerID, petID string, page authBaseDomain.PageRequest) (authBaseDomain.Page[domain.ScanWithMessages], error) {
	if !isPetID(petID) {
		return authBaseDomain.Page[domain.ScanWithMessages]{}, domain.ErrNotFound
	}
	if _, err := s.pets.GetForOwner(ctx, ownerID, petID); err != nil {
		return authBaseDomain.Page[domain.ScanWithMessages]{}, err
	}
	return s.scans.ListScansByPet(ctx, ownerID, petID, page)
}

func (s *PublicTagService) tagByCode(ctx context.Context, code string) (domain.PetTag, error) {
	normalized, ok := domain.NormalizeTagCode(code)
	if !ok {
		return domain.PetTag{}, domain.ErrNotFound
	}
	return s.tags.GetByCode(ctx, normalized)
}

// activeTagAndPet returns the tag and its pet, or ErrNotFound unless the tag is
// active and its pet still exists.
func (s *PublicTagService) activeTagAndPet(ctx context.Context, code string) (domain.PetTag, domain.Pet, error) {
	tag, err := s.tagByCode(ctx, code)
	if err != nil {
		return domain.PetTag{}, domain.Pet{}, err
	}
	if tag.Status != domain.TagStatusActive {
		return domain.PetTag{}, domain.Pet{}, domain.ErrNotFound
	}
	pet, err := s.pets.GetForOwner(ctx, tag.OwnerID, tag.PetID)
	if err != nil {
		return domain.PetTag{}, domain.Pet{}, err
	}
	return tag, pet, nil
}

// authorizedScan validates the scan token in constant time and checks the tag
// is still active and linked to the scan's pet, which must still exist.
func (s *PublicTagService) authorizedScan(ctx context.Context, scanID, token string) (domain.TagScan, domain.PetTag, domain.Pet, error) {
	fail := func(err error) (domain.TagScan, domain.PetTag, domain.Pet, error) {
		return domain.TagScan{}, domain.PetTag{}, domain.Pet{}, err
	}
	if !isPetID(scanID) {
		return fail(domain.ErrNotFound)
	}
	scan, err := s.scans.GetScan(ctx, scanID)
	if err != nil {
		return fail(err)
	}
	if token == "" || subtle.ConstantTimeCompare([]byte(domain.KeyedHash(s.pepper, token)), []byte(scan.ScanTokenHash)) != 1 {
		return fail(domain.ErrInvalidScanToken)
	}
	tag, err := s.tags.GetByID(ctx, scan.TagID)
	if err != nil {
		return fail(err)
	}
	if tag.Status != domain.TagStatusActive || tag.PetID != scan.PetID {
		return fail(domain.ErrNotFound)
	}
	pet, err := s.pets.GetForOwner(ctx, tag.OwnerID, tag.PetID)
	if err != nil {
		return fail(err)
	}
	return scan, tag, pet, nil
}

// whatsAppURL builds a wa.me link from the digits of phone. A 9-digit number
// starting with 9 is a Chilean mobile and gets the 56 country prefix.
func whatsAppURL(phone string) string {
	var digits strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	if len(d) == 9 && strings.HasPrefix(d, "9") {
		d = "56" + d
	}
	return "https://wa.me/" + d
}
