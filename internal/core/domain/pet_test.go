package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var petNow = time.Date(2026, 10, 18, 15, 0, 0, 0, time.UTC)

func validPetDetails() PetDetails {
	birth := time.Date(2021, 3, 14, 0, 0, 0, 0, time.UTC)
	weight := 12.5
	return PetDetails{
		Name:       "Luna",
		Species:    PetSpeciesDog,
		Breed:      "Mestiza",
		Sex:        PetSexFemale,
		BirthDate:  &birth,
		ChipNumber: "900123456789012",
		WeightKg:   &weight,
		Size:       PetSizeMedium,
		Owner: PetOwnerContact{
			FullName: "Wilson Sayago",
			Comuna:   "Ñuñoa",
			Phone:    "+56 9 1234 5678",
		},
	}
}

func issueCodes(t *testing.T, err error) map[string]string {
	t.Helper()
	var invalid *PetValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("expected PetValidationError, got %v", err)
	}
	if !errors.Is(err, ErrInvalidPet) {
		t.Fatalf("validation error must wrap ErrInvalidPet")
	}
	out := map[string]string{}
	for _, issue := range invalid.Issues {
		out[issue.Field] = issue.Code
	}
	return out
}

func TestValidPetDetailsPass(t *testing.T) {
	details := NormalizePetDetails(validPetDetails())
	if err := ValidatePetDetails(details, petNow); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizePetDetailsTrimsAndDefaults(t *testing.T) {
	in := validPetDetails()
	in.Name = "  Luna   la  Bella "
	in.Species = " DOG "
	in.Sex = ""
	in.ChipNumber = "900 123-456.789012"
	in.Health.Allergies = []string{" atún ", "", "Atún", "pollo"}
	in.PublicVisibility = map[PetPublicField]PetVisibility{PetPublicBreed: " ALWAYS "}

	out := NormalizePetDetails(in)
	if out.Name != "Luna la Bella" || out.Species != PetSpeciesDog || out.Sex != PetSexUnknown {
		t.Fatalf("normalized=%+v", out)
	}
	if out.ChipNumber != "900123456789012" {
		t.Fatalf("chip=%q", out.ChipNumber)
	}
	if strings.Join(out.Health.Allergies, "|") != "atún|pollo" {
		t.Fatalf("allergies=%v", out.Health.Allergies)
	}
	if out.PublicVisibility[PetPublicBreed] != PetVisibilityAlways {
		t.Fatalf("breed visibility=%q", out.PublicVisibility[PetPublicBreed])
	}
	if out.PublicVisibility[PetPublicComuna] != PetVisibilityLostOnly {
		t.Fatalf("defaults must fill missing keys, comuna=%q", out.PublicVisibility[PetPublicComuna])
	}
	if len(out.PublicVisibility) != len(PetPublicFields()) {
		t.Fatalf("visibility keys=%d", len(out.PublicVisibility))
	}
}

func TestDefaultVisibilityFollowsBrandManual(t *testing.T) {
	defaults := DefaultPetVisibility()
	want := map[PetPublicField]PetVisibility{
		PetPublicPhoto:             PetVisibilityAlways,
		PetPublicAllergies:         PetVisibilityAlways,
		PetPublicSpecialConditions: PetVisibilityAlways,
		PetPublicContactButton:     PetVisibilityAlways,
		PetPublicOwnerName:         PetVisibilityLostOnly,
		PetPublicComuna:            PetVisibilityLostOnly,
		PetPublicBreed:             PetVisibilityLostOnly,
	}
	for field, visibility := range want {
		if defaults[field] != visibility {
			t.Fatalf("%s=%q want %q", field, defaults[field], visibility)
		}
	}
	for _, private := range []PetPublicField{"address", "chipNumber", "phone", "birthDate", "weightKg"} {
		if _, ok := defaults[private]; ok {
			t.Fatalf("%s must never be a public field", private)
		}
	}
	defaults[PetPublicPhoto] = PetVisibilityHidden
	if DefaultPetVisibility()[PetPublicPhoto] != PetVisibilityAlways {
		t.Fatal("DefaultPetVisibility must return a copy")
	}
}

func TestValidatePetDetailsReportsEveryField(t *testing.T) {
	in := validPetDetails()
	in.Name = ""
	in.Species = "dragon"
	in.ChipNumber = "12AB"
	tooHeavy := 151.0
	in.WeightKg = &tooHeavy
	future := petNow.AddDate(0, 0, 2)
	in.BirthDate = &future
	in.Owner.FullName = ""
	in.Owner.Phone = "123"
	in.Owner.PhoneAlt = "+56 9 abc"
	in.SecondOwner = PetSecondOwner{Phone: "+56 9 8765 4321"}
	in.PublicVisibility = map[PetPublicField]PetVisibility{"address": PetVisibilityAlways, PetPublicBreed: "sometimes"}

	codes := issueCodes(t, ValidatePetDetails(NormalizePetDetails(in), petNow))
	want := map[string]string{
		"name":                     "required",
		"species":                  "oneof",
		"chipNumber":               "digits",
		"weightKg":                 "range",
		"birthDate":                "future",
		"owner.fullName":           "required",
		"owner.phone":              "phone",
		"owner.phoneAlt":           "phone",
		"secondOwner.fullName":     "required",
		"publicVisibility.address": "unknown",
		"publicVisibility.breed":   "oneof",
	}
	for field, code := range want {
		if codes[field] != code {
			t.Fatalf("%s=%q want %q (all=%v)", field, codes[field], code, codes)
		}
	}
}

func TestValidatePetDetailsLimits(t *testing.T) {
	in := validPetDetails()
	in.ChipNumber = "12345678"
	in.Name = strings.Repeat("a", MaxPetNameLength+1)
	in.Health.Allergies = make([]string, MaxPetHealthItems+1)
	for i := range in.Health.Allergies {
		in.Health.Allergies[i] = strings.Repeat("x", i+1)
	}
	in.Health.Surgeries = []string{strings.Repeat("y", MaxPetHealthItemLength+1)}
	old := time.Date(1979, 12, 31, 0, 0, 0, 0, time.UTC)
	in.BirthDate = &old

	codes := issueCodes(t, ValidatePetDetails(NormalizePetDetails(in), petNow))
	for field, code := range map[string]string{
		"chipNumber":       "length",
		"name":             "max",
		"health.allergies": "max_items",
		"health.surgeries": "item",
		"birthDate":        "too_old",
	} {
		if codes[field] != code {
			t.Fatalf("%s=%q want %q (all=%v)", field, codes[field], code, codes)
		}
	}
}

func TestSecondOwnerIsOptional(t *testing.T) {
	in := validPetDetails()
	in.SecondOwner = PetSecondOwner{FullName: "  ", Phone: ""}
	if err := ValidatePetDetails(NormalizePetDetails(in), petNow); err != nil {
		t.Fatalf("empty second owner must be valid: %v", err)
	}
}

func TestPetBirthDateRoundTrip(t *testing.T) {
	date, err := ParsePetBirthDate("2021-03-14")
	if err != nil || FormatPetBirthDate(date) != "2021-03-14" {
		t.Fatalf("date=%v err=%v", date, err)
	}
	if date, err := ParsePetBirthDate(" "); err != nil || date != nil {
		t.Fatalf("empty must be nil: %v %v", date, err)
	}
	if _, err := ParsePetBirthDate("14/03/2021"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestPetWeightIsRoundedToGrams(t *testing.T) {
	in := validPetDetails()
	weight := 12.567
	in.WeightKg = &weight
	out := NormalizePetDetails(in)
	if out.WeightKg == nil || *out.WeightKg != 12.57 {
		t.Fatalf("weight=%v, want 12.57 (the column stores 2 decimals)", out.WeightKg)
	}
}

func TestPetWeightBelowColumnPrecisionIsInvalid(t *testing.T) {
	in := validPetDetails()
	tiny := 0.004
	in.WeightKg = &tiny
	codes := issueCodes(t, ValidatePetDetails(NormalizePetDetails(in), petNow))
	if codes["weightKg"] != "range" {
		t.Fatalf("0.004 kg must be rejected instead of becoming 0.00 in the database: %v", codes)
	}
}
