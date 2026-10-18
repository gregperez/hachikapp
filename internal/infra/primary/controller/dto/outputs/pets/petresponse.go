package pets

// PetResponse is a pet record as seen by its owner.
type PetResponse struct {
	ID                  string                 `json:"id" example:"2f1c5a0e-6a43-4f8e-9a51-0c2f3f9b7d11"`
	Name                string                 `json:"name" example:"Luna"`
	Species             string                 `json:"species" example:"dog"`
	Breed               string                 `json:"breed" example:"Mestiza"`
	Color               string                 `json:"color" example:"Café con blanco"`
	Sex                 string                 `json:"sex" example:"female"`
	BirthDate           string                 `json:"birthDate" example:"2021-03-14"`
	ChipNumber          string                 `json:"chipNumber" example:"900123456789012"`
	WeightKg            *float64               `json:"weightKg" example:"12.5"`
	Size                string                 `json:"size" example:"medium"`
	DistinctiveFeatures string                 `json:"distinctiveFeatures"`
	Health              PetHealthResponse      `json:"health"`
	Owner               PetOwnerResponse       `json:"owner"`
	SecondOwner         PetSecondOwnerResponse `json:"secondOwner"`
	PublicVisibility    map[string]string      `json:"publicVisibility"`
	PhotoURL            string                 `json:"photoUrl,omitempty" example:"/v1/pets/2f1c5a0e-6a43-4f8e-9a51-0c2f3f9b7d11/photo?v=1760820000000"`
	Version             int64                  `json:"version" example:"1"`
	CreatedAt           string                 `json:"createdAt" example:"2026-10-18T21:00:00Z"`
	UpdatedAt           string                 `json:"updatedAt" example:"2026-10-18T21:00:00Z"`
}

type PetHealthResponse struct {
	Allergies         []string `json:"allergies"`
	Surgeries         []string `json:"surgeries"`
	SpecialConditions []string `json:"specialConditions"`
	Temperament       string   `json:"temperament"`
}

type PetOwnerResponse struct {
	FullName string `json:"fullName"`
	Address  string `json:"address"`
	Comuna   string `json:"comuna"`
	Phone    string `json:"phone"`
	PhoneAlt string `json:"phoneAlt"`
}

type PetSecondOwnerResponse struct {
	FullName string `json:"fullName"`
	Phone    string `json:"phone"`
}
