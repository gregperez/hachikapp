package domain

import (
	"strings"
	"time"
	"unicode"
)

// PublicProfile is the projection of a pet shown to anyone who scans its tag.
// It is built only from fields the owner made visible and never carries
// phones, addresses, chip numbers or surnames.
type PublicProfile struct {
	Name                 string
	HasPhoto             bool
	PhotoVersion         int64
	Species              string
	Breed                string
	Color                string
	Sex                  string
	Size                 string
	DistinctiveFeatures  string
	Temperament          string
	AgeMonths            *int
	Allergies            []string
	SpecialConditions    []string
	Surgeries            []string
	OwnerFirstName       string
	SecondOwnerFirstName string
	Comuna               string
	CanContact           bool
	Lost                 bool
}

// FirstName returns the first whitespace-separated token of a full name.
func FirstName(full string) string {
	fields := strings.Fields(full)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// ComunaIsSafe reports whether a comuna contains no digits, so it cannot
// smuggle a street number or phone into the public page.
func ComunaIsSafe(value string) bool {
	for _, r := range value {
		if unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// PublicProfileOf projects a pet to what its visibility settings allow.
func PublicProfileOf(pet Pet, lost bool, now time.Time) PublicProfile {
	defaults := DefaultPetVisibility()
	visible := func(field PetPublicField) bool {
		v, ok := pet.PublicVisibility[field]
		if !ok {
			v = defaults[field]
		}
		return v == PetVisibilityAlways || (v == PetVisibilityLostOnly && lost)
	}
	text := func(field PetPublicField, value string) string {
		if visible(field) {
			return value
		}
		return ""
	}
	list := func(field PetPublicField, values []string) []string {
		if !visible(field) || len(values) == 0 {
			return nil
		}
		return append([]string(nil), values...)
	}

	p := PublicProfile{
		Name:                pet.Name,
		Species:             text(PetPublicSpecies, string(pet.Species)),
		Breed:               text(PetPublicBreed, pet.Breed),
		Color:               text(PetPublicColor, pet.Color),
		Sex:                 text(PetPublicSex, string(pet.Sex)),
		Size:                text(PetPublicSize, string(pet.Size)),
		DistinctiveFeatures: text(PetPublicDistinctiveFeatures, pet.DistinctiveFeatures),
		Temperament:         text(PetPublicTemperament, pet.Health.Temperament),
		Allergies:           list(PetPublicAllergies, pet.Health.Allergies),
		SpecialConditions:   list(PetPublicSpecialConditions, pet.Health.SpecialConditions),
		Surgeries:           list(PetPublicSurgeries, pet.Health.Surgeries),
		CanContact:          visible(PetPublicContactButton),
		Lost:                lost,
	}
	if pet.HasPhoto && visible(PetPublicPhoto) {
		p.HasPhoto = true
		p.PhotoVersion = pet.PhotoUpdatedAt.UnixMilli()
	}
	if visible(PetPublicOwnerName) {
		p.OwnerFirstName = FirstName(pet.Owner.FullName)
	}
	if visible(PetPublicSecondOwner) {
		p.SecondOwnerFirstName = FirstName(pet.SecondOwner.FullName)
	}
	if visible(PetPublicComuna) && ComunaIsSafe(pet.Owner.Comuna) {
		p.Comuna = strings.TrimSpace(pet.Owner.Comuna)
	}
	if visible(PetPublicAge) && pet.BirthDate != nil {
		months := wholeMonths(*pet.BirthDate, now)
		p.AgeMonths = &months
	}
	return p
}

// wholeMonths counts complete months from birth to now using UTC dates.
func wholeMonths(birth, now time.Time) int {
	b, n := birth.UTC(), now.UTC()
	months := (n.Year()-b.Year())*12 + int(n.Month()) - int(b.Month())
	if n.Day() < b.Day() {
		months--
	}
	if months < 0 {
		return 0
	}
	return months
}
