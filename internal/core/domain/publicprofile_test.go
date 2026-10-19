package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func publicTestPet() Pet {
	details := NormalizePetDetails(validPetDetails())
	details.Breed = "Mestiza"
	details.Color = "Café"
	details.Sex = PetSexFemale
	details.Size = PetSizeMedium
	details.DistinctiveFeatures = "Mancha blanca"
	details.Health = PetHealth{
		Allergies:         []string{"Pollo"},
		Surgeries:         []string{"Esterilización"},
		SpecialConditions: []string{"Sorda"},
		Temperament:       "Tímida",
	}
	details.Owner.Address = "Calle Secreta 999"
	details.Owner.PhoneAlt = "+56 9 8765 4321"
	details.PublicVisibility = DefaultPetVisibility()
	return Pet{
		ID:             "pet-1",
		OwnerID:        "acc-1",
		PetDetails:     details,
		HasPhoto:       true,
		PhotoUpdatedAt: time.UnixMilli(1700000000123),
	}
}

func TestPublicProfileDefaultsHideLostOnly(t *testing.T) {
	p := PublicProfileOf(publicTestPet(), false, petNow)
	if p.Name != "Luna" || !p.HasPhoto || p.PhotoVersion != 1700000000123 || p.Species != "dog" {
		t.Fatalf("visible fields missing: %+v", p)
	}
	if len(p.Allergies) != 1 || len(p.SpecialConditions) != 1 || p.Temperament != "Tímida" {
		t.Fatalf("health defaults missing: %+v", p)
	}
	if !p.CanContact || p.Lost {
		t.Fatalf("contact/lost wrong: %+v", p)
	}
	if p.Breed != "" || p.Color != "" || p.Sex != "" || p.Size != "" || p.DistinctiveFeatures != "" ||
		p.Comuna != "" || p.OwnerFirstName != "" || p.SecondOwnerFirstName != "" || p.AgeMonths != nil {
		t.Fatalf("hidden fields leaked: %+v", p)
	}
}

func TestPublicProfileLostShowsLostOnly(t *testing.T) {
	p := PublicProfileOf(publicTestPet(), true, petNow)
	if !p.Lost || p.Breed != "Mestiza" || p.Comuna != "Ñuñoa" || p.OwnerFirstName != "Wilson" {
		t.Fatalf("lost_only fields missing: %+v", p)
	}
}

func TestPublicProfileHonoursHidden(t *testing.T) {
	pet := publicTestPet()
	pet.PublicVisibility[PetPublicAllergies] = PetVisibilityHidden
	pet.PublicVisibility[PetPublicContactButton] = PetVisibilityHidden
	pet.PublicVisibility[PetPublicPhoto] = PetVisibilityHidden
	p := PublicProfileOf(pet, true, petNow)
	if len(p.Allergies) != 0 || p.CanContact || p.HasPhoto || p.PhotoVersion != 0 {
		t.Fatalf("hidden not honoured: %+v", p)
	}
}

func TestPublicProfileMissingKeyUsesDefault(t *testing.T) {
	pet := publicTestPet()
	pet.PublicVisibility = map[PetPublicField]PetVisibility{}
	p := PublicProfileOf(pet, false, petNow)
	if p.Species != "dog" || !p.CanContact || p.Breed != "" {
		t.Fatalf("defaults not applied: %+v", p)
	}
}

func TestPublicProfileOwnerFirstNameOnly(t *testing.T) {
	pet := publicTestPet()
	pet.SecondOwner = PetSecondOwner{FullName: "Andrea Alastre", Phone: "+56 9 1111 2222"}
	pet.PublicVisibility[PetPublicSecondOwner] = PetVisibilityAlways
	p := PublicProfileOf(pet, true, petNow)
	if p.OwnerFirstName != "Wilson" || p.SecondOwnerFirstName != "Andrea" {
		t.Fatalf("got %q / %q", p.OwnerFirstName, p.SecondOwnerFirstName)
	}
}

func TestPublicProfileDropsUnsafeComuna(t *testing.T) {
	pet := publicTestPet()
	pet.Owner.Comuna = "Av X 123"
	pet.PublicVisibility[PetPublicComuna] = PetVisibilityAlways
	if got := PublicProfileOf(pet, true, petNow).Comuna; got != "" {
		t.Fatalf("unsafe comuna leaked: %q", got)
	}
}

func TestPublicProfileAgeMonths(t *testing.T) {
	pet := publicTestPet()
	pet.PublicVisibility[PetPublicAge] = PetVisibilityAlways
	p := PublicProfileOf(pet, false, petNow)
	if p.AgeMonths == nil || *p.AgeMonths != 67 {
		t.Fatalf("age months = %v", p.AgeMonths)
	}
	future := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	pet.BirthDate = &future
	if got := PublicProfileOf(pet, false, petNow).AgeMonths; got == nil || *got != 0 {
		t.Fatalf("future birth must clamp to 0, got %v", got)
	}
	pet.BirthDate = nil
	if PublicProfileOf(pet, false, petNow).AgeMonths != nil {
		t.Fatal("no birth date must give nil age")
	}
}

func TestFirstName(t *testing.T) {
	cases := map[string]string{"": "", "   ": "", " Wilson  Sayago ": "Wilson", "Ana": "Ana"}
	for in, want := range cases {
		if got := FirstName(in); got != want {
			t.Errorf("FirstName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPublicProfileNeverCarriesPrivateData(t *testing.T) {
	pet := publicTestPet()
	pet.SecondOwner = PetSecondOwner{FullName: "Andrea Alastre", Phone: "+56 9 1111 2222"}
	for field := range pet.PublicVisibility {
		pet.PublicVisibility[field] = PetVisibilityAlways
	}
	raw, err := json.Marshal(PublicProfileOf(pet, true, petNow))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, secret := range []string{"1234 5678", "8765 4321", "1111 2222", "Calle Secreta", "999", "900123456789012", "Sayago", "Alastre"} {
		if strings.Contains(body, secret) {
			t.Errorf("profile leaks %q: %s", secret, body)
		}
	}
}
