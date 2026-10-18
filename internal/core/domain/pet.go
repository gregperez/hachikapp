package domain

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Pet field limits. They mirror the CHECK constraints of migration 000017.
const (
	MaxPetNameLength            = 60
	MaxPetBreedLength           = 60
	MaxPetColorLength           = 60
	MaxPetFeaturesLength        = 500
	MaxPetTemperamentLength     = 300
	MaxPetHealthItemLength      = 120
	MaxPetHealthItems           = 20
	MaxPetOwnerNameLength       = 100
	MaxPetAddressLength         = 200
	MaxPetComunaLength          = 60
	MaxPetPhoneLength           = 20
	MinPetPhoneDigits           = 8
	MinPetChipDigits            = 9
	MaxPetChipDigits            = 15
	MaxPetWeightKg              = 150.0
	MaxPetsPerOwner             = 50
	MaxPetPhotoBytes            = 1 << 20
	earliestPetBirthYear        = 1980
	petBirthDateLayout          = "2006-01-02"
	petFieldName                = "name"
	petFieldOwnerName           = "owner.fullName"
	petFieldOwnerPhone          = "owner.phone"
	petFieldOwnerPhoneAlt       = "owner.phoneAlt"
	petFieldSecondOwnerName     = "secondOwner.fullName"
	petFieldSecondOwnerPhone    = "secondOwner.phone"
	petFieldVisibilityKeyPrefix = "publicVisibility."
)

// ErrInvalidPet is the sentinel wrapped by PetValidationError (maps to HTTP 422).
var ErrInvalidPet = errors.New("invalid pet")

// ErrDuplicateChip is returned when another pet already uses the chip number (maps to HTTP 409).
var ErrDuplicateChip = errors.New("chip number already registered")

// ErrPetLimitReached is returned when an owner already has MaxPetsPerOwner pets (maps to HTTP 409).
var ErrPetLimitReached = errors.New("pet limit reached")

// ErrInvalidPetPhoto is returned for empty, oversized or unsupported images (maps to HTTP 422).
var ErrInvalidPetPhoto = errors.New("invalid pet photo")

// PetFieldIssue identifies one invalid field with a stable machine code.
type PetFieldIssue struct {
	Field string
	Code  string
}

// PetValidationError lists every invalid field of a pet payload.
type PetValidationError struct {
	Issues []PetFieldIssue
}

func (e *PetValidationError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return ErrInvalidPet.Error()
	}
	parts := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		parts = append(parts, issue.Field+":"+issue.Code)
	}
	return fmt.Sprintf("%s (%s)", ErrInvalidPet.Error(), strings.Join(parts, ", "))
}

func (e *PetValidationError) Unwrap() error { return ErrInvalidPet }

type PetSpecies string

const (
	PetSpeciesDog   PetSpecies = "dog"
	PetSpeciesCat   PetSpecies = "cat"
	PetSpeciesOther PetSpecies = "other"
)

type PetSex string

const (
	PetSexMale    PetSex = "male"
	PetSexFemale  PetSex = "female"
	PetSexUnknown PetSex = "unknown"
)

type PetSize string

const (
	PetSizeUnset  PetSize = ""
	PetSizeSmall  PetSize = "small"
	PetSizeMedium PetSize = "medium"
	PetSizeLarge  PetSize = "large"
	PetSizeGiant  PetSize = "giant"
)

// PetVisibility controls when a field appears on the public profile reached
// through the Hachikapp Tag.
type PetVisibility string

const (
	PetVisibilityHidden   PetVisibility = "hidden"
	PetVisibilityLostOnly PetVisibility = "lost_only"
	PetVisibilityAlways   PetVisibility = "always"
)

// PetPublicField is a field the owner may expose on the public profile.
// The exact address, chip number, exact birth date and weight are deliberately
// absent: they are never public (brand manual v2, section 5).
type PetPublicField string

const (
	PetPublicPhoto               PetPublicField = "photo"
	PetPublicSpecies             PetPublicField = "species"
	PetPublicBreed               PetPublicField = "breed"
	PetPublicColor               PetPublicField = "color"
	PetPublicSex                 PetPublicField = "sex"
	PetPublicAge                 PetPublicField = "age"
	PetPublicSize                PetPublicField = "size"
	PetPublicDistinctiveFeatures PetPublicField = "distinctiveFeatures"
	PetPublicAllergies           PetPublicField = "allergies"
	PetPublicSpecialConditions   PetPublicField = "specialConditions"
	PetPublicSurgeries           PetPublicField = "surgeries"
	PetPublicTemperament         PetPublicField = "temperament"
	PetPublicOwnerName           PetPublicField = "ownerName"
	PetPublicContactButton       PetPublicField = "contactButton"
	PetPublicComuna              PetPublicField = "comuna"
	PetPublicSecondOwner         PetPublicField = "secondOwner"
)

// defaultPetVisibility follows the recommended defaults of the brand manual:
// photo, health alerts and the contact button (never the number) are always
// visible; identifying data that eases scams or targeted theft only shows in
// lost mode; the rest stays hidden.
var defaultPetVisibility = map[PetPublicField]PetVisibility{
	PetPublicPhoto:               PetVisibilityAlways,
	PetPublicSpecies:             PetVisibilityAlways,
	PetPublicBreed:               PetVisibilityLostOnly,
	PetPublicColor:               PetVisibilityLostOnly,
	PetPublicSex:                 PetVisibilityLostOnly,
	PetPublicAge:                 PetVisibilityHidden,
	PetPublicSize:                PetVisibilityLostOnly,
	PetPublicDistinctiveFeatures: PetVisibilityLostOnly,
	PetPublicAllergies:           PetVisibilityAlways,
	PetPublicSpecialConditions:   PetVisibilityAlways,
	PetPublicSurgeries:           PetVisibilityHidden,
	PetPublicTemperament:         PetVisibilityAlways,
	PetPublicOwnerName:           PetVisibilityLostOnly,
	PetPublicContactButton:       PetVisibilityAlways,
	PetPublicComuna:              PetVisibilityLostOnly,
	PetPublicSecondOwner:         PetVisibilityHidden,
}

// PetPublicFields returns the allowlisted public fields in a stable order.
func PetPublicFields() []PetPublicField {
	return []PetPublicField{
		PetPublicPhoto, PetPublicSpecies, PetPublicBreed, PetPublicColor, PetPublicSex,
		PetPublicAge, PetPublicSize, PetPublicDistinctiveFeatures, PetPublicAllergies,
		PetPublicSpecialConditions, PetPublicSurgeries, PetPublicTemperament,
		PetPublicOwnerName, PetPublicContactButton, PetPublicComuna, PetPublicSecondOwner,
	}
}

// DefaultPetVisibility returns a fresh copy of the recommended defaults.
func DefaultPetVisibility() map[PetPublicField]PetVisibility {
	out := make(map[PetPublicField]PetVisibility, len(defaultPetVisibility))
	for field, visibility := range defaultPetVisibility {
		out[field] = visibility
	}
	return out
}

// PetOwnerContact is the owner registered with the microchip. It is contact
// data on the pet record; the managing account is Pet.OwnerID.
type PetOwnerContact struct {
	FullName string
	Address  string
	Comuna   string
	Phone    string
	PhoneAlt string
}

// PetSecondOwner is an optional secondary contact.
type PetSecondOwner struct {
	FullName string
	Phone    string
}

// IsEmpty reports whether no second owner data was provided.
func (s PetSecondOwner) IsEmpty() bool {
	return strings.TrimSpace(s.FullName) == "" && strings.TrimSpace(s.Phone) == ""
}

// PetHealth groups the health conditions shown on the record.
type PetHealth struct {
	Allergies         []string
	Surgeries         []string
	SpecialConditions []string
	Temperament       string
}

// PetDetails is the owner-editable content of a pet record.
type PetDetails struct {
	Name                string
	Species             PetSpecies
	Breed               string
	Color               string
	Sex                 PetSex
	BirthDate           *time.Time
	ChipNumber          string
	WeightKg            *float64
	Size                PetSize
	DistinctiveFeatures string
	Health              PetHealth
	Owner               PetOwnerContact
	SecondOwner         PetSecondOwner
	PublicVisibility    map[PetPublicField]PetVisibility
}

// Pet is a pet record managed by one account.
type Pet struct {
	ID      string
	OwnerID string
	PetDetails
	HasPhoto       bool
	PhotoUpdatedAt time.Time
	Version        int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// PetPhoto is an image attached to a pet.
type PetPhoto struct {
	ContentType string
	Data        []byte
	UpdatedAt   time.Time
}

// ParsePetBirthDate parses a YYYY-MM-DD date. Empty input returns nil.
func ParsePetBirthDate(raw string) (*time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(petBirthDateLayout, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// FormatPetBirthDate renders a birth date as YYYY-MM-DD, or "" when unset.
func FormatPetBirthDate(date *time.Time) string {
	if date == nil || date.IsZero() {
		return ""
	}
	return date.UTC().Format(petBirthDateLayout)
}

// NormalizePetDetails trims text, drops empty or duplicated health items,
// fills enum defaults and merges visibility with the recommended defaults.
func NormalizePetDetails(in PetDetails) PetDetails {
	out := in
	out.Name = collapseSpaces(in.Name)
	out.Species = PetSpecies(strings.ToLower(strings.TrimSpace(string(in.Species))))
	out.Breed = collapseSpaces(in.Breed)
	out.Color = collapseSpaces(in.Color)
	out.Sex = PetSex(strings.ToLower(strings.TrimSpace(string(in.Sex))))
	if out.Sex == "" {
		out.Sex = PetSexUnknown
	}
	if in.BirthDate != nil {
		date := time.Date(in.BirthDate.Year(), in.BirthDate.Month(), in.BirthDate.Day(), 0, 0, 0, 0, time.UTC)
		out.BirthDate = &date
	}
	out.ChipNumber = stripChipSeparators(in.ChipNumber)
	out.Size = PetSize(strings.ToLower(strings.TrimSpace(string(in.Size))))
	out.DistinctiveFeatures = strings.TrimSpace(in.DistinctiveFeatures)
	out.Health = PetHealth{
		Allergies:         normalizeItems(in.Health.Allergies),
		Surgeries:         normalizeItems(in.Health.Surgeries),
		SpecialConditions: normalizeItems(in.Health.SpecialConditions),
		Temperament:       strings.TrimSpace(in.Health.Temperament),
	}
	out.Owner = PetOwnerContact{
		FullName: collapseSpaces(in.Owner.FullName),
		Address:  collapseSpaces(in.Owner.Address),
		Comuna:   collapseSpaces(in.Owner.Comuna),
		Phone:    collapseSpaces(in.Owner.Phone),
		PhoneAlt: collapseSpaces(in.Owner.PhoneAlt),
	}
	out.SecondOwner = PetSecondOwner{
		FullName: collapseSpaces(in.SecondOwner.FullName),
		Phone:    collapseSpaces(in.SecondOwner.Phone),
	}
	visibility := DefaultPetVisibility()
	for field, value := range in.PublicVisibility {
		visibility[field] = PetVisibility(strings.ToLower(strings.TrimSpace(string(value))))
	}
	out.PublicVisibility = visibility
	return out
}

// ValidatePetDetails reports every invalid field of normalized details.
// The now argument anchors the birth-date check.
func ValidatePetDetails(details PetDetails, now time.Time) error {
	var issues []PetFieldIssue
	add := func(field, code string) { issues = append(issues, PetFieldIssue{Field: field, Code: code}) }

	checkText(details.Name, petFieldName, MaxPetNameLength, true, add)
	switch details.Species {
	case PetSpeciesDog, PetSpeciesCat, PetSpeciesOther:
	case "":
		add("species", "required")
	default:
		add("species", "oneof")
	}
	checkText(details.Breed, "breed", MaxPetBreedLength, false, add)
	checkText(details.Color, "color", MaxPetColorLength, false, add)
	switch details.Sex {
	case PetSexMale, PetSexFemale, PetSexUnknown:
	default:
		add("sex", "oneof")
	}
	if details.BirthDate != nil {
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		switch {
		case details.BirthDate.After(today):
			add("birthDate", "future")
		case details.BirthDate.Year() < earliestPetBirthYear:
			add("birthDate", "too_old")
		}
	}
	if details.ChipNumber != "" {
		if !isDigits(details.ChipNumber) {
			add("chipNumber", "digits")
		} else if n := len(details.ChipNumber); n < MinPetChipDigits || n > MaxPetChipDigits {
			add("chipNumber", "length")
		}
	}
	if details.WeightKg != nil && (*details.WeightKg <= 0 || *details.WeightKg > MaxPetWeightKg) {
		add("weightKg", "range")
	}
	switch details.Size {
	case PetSizeUnset, PetSizeSmall, PetSizeMedium, PetSizeLarge, PetSizeGiant:
	default:
		add("size", "oneof")
	}
	checkText(details.DistinctiveFeatures, "distinctiveFeatures", MaxPetFeaturesLength, false, add)
	checkItems(details.Health.Allergies, "health.allergies", add)
	checkItems(details.Health.Surgeries, "health.surgeries", add)
	checkItems(details.Health.SpecialConditions, "health.specialConditions", add)
	checkText(details.Health.Temperament, "health.temperament", MaxPetTemperamentLength, false, add)

	checkText(details.Owner.FullName, petFieldOwnerName, MaxPetOwnerNameLength, true, add)
	checkText(details.Owner.Address, "owner.address", MaxPetAddressLength, false, add)
	checkText(details.Owner.Comuna, "owner.comuna", MaxPetComunaLength, false, add)
	checkPhone(details.Owner.Phone, petFieldOwnerPhone, true, add)
	checkPhone(details.Owner.PhoneAlt, petFieldOwnerPhoneAlt, false, add)

	if !details.SecondOwner.IsEmpty() {
		checkText(details.SecondOwner.FullName, petFieldSecondOwnerName, MaxPetOwnerNameLength, true, add)
		checkPhone(details.SecondOwner.Phone, petFieldSecondOwnerPhone, false, add)
	}

	for field, value := range details.PublicVisibility {
		key := petFieldVisibilityKeyPrefix + string(field)
		if _, ok := defaultPetVisibility[field]; !ok {
			add(key, "unknown")
			continue
		}
		switch value {
		case PetVisibilityHidden, PetVisibilityLostOnly, PetVisibilityAlways:
		default:
			add(key, "oneof")
		}
	}

	if len(issues) == 0 {
		return nil
	}
	sortIssues(issues)
	return &PetValidationError{Issues: issues}
}

func checkText(value, field string, maxLen int, required bool, add func(string, string)) {
	if value == "" {
		if required {
			add(field, "required")
		}
		return
	}
	if !utf8.ValidString(value) || hasControl(value) {
		add(field, "invalid")
		return
	}
	if utf8.RuneCountInString(value) > maxLen {
		add(field, "max")
	}
}

func checkItems(items []string, field string, add func(string, string)) {
	if len(items) > MaxPetHealthItems {
		add(field, "max_items")
		return
	}
	for _, item := range items {
		if hasControl(item) || utf8.RuneCountInString(item) > MaxPetHealthItemLength {
			add(field, "item")
			return
		}
	}
}

func checkPhone(value, field string, required bool, add func(string, string)) {
	if value == "" {
		if required {
			add(field, "required")
		}
		return
	}
	if utf8.RuneCountInString(value) > MaxPetPhoneLength {
		add(field, "max")
		return
	}
	digits := 0
	for i, r := range value {
		switch {
		case unicode.IsDigit(r) && r < unicode.MaxASCII:
			digits++
		case r == '+' && i == 0, r == ' ', r == '-', r == '(', r == ')':
		default:
			add(field, "phone")
			return
		}
	}
	if digits < MinPetPhoneDigits {
		add(field, "phone")
	}
}

func normalizeItems(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		value := collapseSpaces(item)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func collapseSpaces(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func stripChipSeparators(value string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '-' || r == '.' {
			return -1
		}
		return r
	}, strings.TrimSpace(value))
}

func isDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return value != ""
}

func hasControl(value string) bool {
	for _, r := range value {
		if r != '\n' && unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func sortIssues(issues []PetFieldIssue) {
	sort.SliceStable(issues, func(i, j int) bool { return issues[i].Field < issues[j].Field })
}
