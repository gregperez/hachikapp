package domain

import (
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

type TagKind string

const (
	TagKindPhysical TagKind = "physical"
	TagKindDigital  TagKind = "digital"
)

type TagStatus string

const (
	TagStatusUnassigned TagStatus = "unassigned"
	TagStatusActive     TagStatus = "active"
	TagStatusDisabled   TagStatus = "disabled"
)

const (
	// TagCodeLength is the number of characters of a tag code.
	TagCodeLength = 10
	// TagCodeAlphabet is Crockford base32 (no I, L, O, U).
	TagCodeAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	// MaxActiveTagsPerPet caps the active tags a pet can have.
	MaxActiveTagsPerPet = 5
	// MaxTagBatchQuantity caps the tags generated in one batch.
	MaxTagBatchQuantity = 1000

	maxTagBatchLabelRunes = 80
)

// ErrTagNotAssignable is returned when a tag cannot be activated by the requester (maps to HTTP 409).
var ErrTagNotAssignable = errors.New("tag cannot be assigned")

// ErrTagLimitReached is returned when a pet already has MaxActiveTagsPerPet active tags (maps to HTTP 409).
var ErrTagLimitReached = errors.New("tag limit reached")

// ErrDuplicateTagCode is returned when a generated tag code already exists.
var ErrDuplicateTagCode = errors.New("duplicate tag code")

type PetTag struct {
	ID          string
	Code        string
	Kind        TagKind
	Status      TagStatus
	BatchID     string
	PetID       string
	OwnerID     string
	ActivatedAt *time.Time
	DisabledAt  *time.Time
	CreatedAt   time.Time
}

type TagBatch struct {
	ID        string
	Label     string
	Quantity  int
	CreatedBy string
	CreatedAt time.Time
}

// NewTagCode builds a random code reading bytes from r. Bytes >= 224 are
// discarded so each alphabet character is equally likely.
func NewTagCode(r io.Reader) (string, error) {
	const limit = 7 * 32
	out := make([]byte, 0, TagCodeLength)
	buf := make([]byte, TagCodeLength)
	for len(out) < TagCodeLength {
		if _, err := io.ReadFull(r, buf); err != nil {
			return "", err
		}
		for _, b := range buf {
			if b >= limit {
				continue
			}
			out = append(out, TagCodeAlphabet[int(b)%len(TagCodeAlphabet)])
			if len(out) == TagCodeLength {
				break
			}
		}
	}
	return string(out), nil
}

// NormalizeTagCode upper-cases raw, strips spaces and hyphens, and maps
// O to 0 and I/L to 1. It reports false when the result is not a valid code.
func NormalizeTagCode(raw string) (string, bool) {
	var b strings.Builder
	for _, c := range strings.ToUpper(raw) {
		switch c {
		case ' ', '-':
			continue
		case 'O':
			c = '0'
		case 'I', 'L':
			c = '1'
		}
		if !strings.ContainsRune(TagCodeAlphabet, c) {
			return "", false
		}
		b.WriteRune(c)
	}
	code := b.String()
	if len(code) != TagCodeLength {
		return "", false
	}
	return code, true
}

// CheckActivatableBy reports whether userID may activate the tag: it must be
// unassigned and either ownerless or already reserved for that user.
func (t PetTag) CheckActivatableBy(userID string) error {
	if t.Status != TagStatusUnassigned {
		return ErrTagNotAssignable
	}
	if t.OwnerID != "" && t.OwnerID != userID {
		return ErrTagNotAssignable
	}
	return nil
}

// ValidateTagBatch checks the label and quantity of a batch to generate.
func ValidateTagBatch(label string, quantity int) error {
	var issues []PetFieldIssue
	label = strings.TrimSpace(label)
	switch {
	case label == "":
		issues = append(issues, PetFieldIssue{Field: "label", Code: "required"})
	case utf8.RuneCountInString(label) > maxTagBatchLabelRunes:
		issues = append(issues, PetFieldIssue{Field: "label", Code: "max"})
	}
	if quantity < 1 || quantity > MaxTagBatchQuantity {
		issues = append(issues, PetFieldIssue{Field: "quantity", Code: "range"})
	}
	if len(issues) == 0 {
		return nil
	}
	return &PetValidationError{Issues: issues}
}
