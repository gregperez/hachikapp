package pets

// PetRequest is the body of POST /v1/pets and PUT /v1/pets/{id}.
// @Description Pet record. Version is required on update.
type PetRequest struct {
	Version             int64                  `json:"version,omitempty" example:"3"`
	Name                string                 `json:"name" example:"Luna"`
	Species             string                 `json:"species" enums:"dog,cat,other" example:"dog"`
	Breed               string                 `json:"breed" example:"Mestiza"`
	Color               string                 `json:"color" example:"Café con blanco"`
	Sex                 string                 `json:"sex" enums:"male,female,unknown" example:"female"`
	BirthDate           string                 `json:"birthDate" example:"2021-03-14"`
	ChipNumber          string                 `json:"chipNumber" example:"900123456789012"`
	WeightKg            *float64               `json:"weightKg" example:"12.5"`
	Size                string                 `json:"size" enums:",small,medium,large,giant" example:"medium"`
	DistinctiveFeatures string                 `json:"distinctiveFeatures" example:"Mancha blanca en el pecho"`
	Health              PetHealthRequest       `json:"health"`
	Owner               PetOwnerRequest        `json:"owner"`
	SecondOwner         *PetSecondOwnerRequest `json:"secondOwner,omitempty"`
	PublicVisibility    map[string]string      `json:"publicVisibility,omitempty"`
}

// PetHealthRequest groups health conditions.
type PetHealthRequest struct {
	Allergies         []string `json:"allergies" example:"Atún"`
	Surgeries         []string `json:"surgeries" example:"Esterilizada"`
	SpecialConditions []string `json:"specialConditions" example:"Sorda"`
	Temperament       string   `json:"temperament" example:"Tímida con extraños"`
}

// PetOwnerRequest is the owner registered with the microchip.
type PetOwnerRequest struct {
	FullName string `json:"fullName" example:"Wilson Sayago"`
	Address  string `json:"address" example:"Av. Siempre Viva 742"`
	Comuna   string `json:"comuna" example:"Ñuñoa"`
	Phone    string `json:"phone" example:"+56 9 1234 5678"`
	PhoneAlt string `json:"phoneAlt" example:""`
}

// PetSecondOwnerRequest is an optional second contact.
type PetSecondOwnerRequest struct {
	FullName string `json:"fullName" example:"Andrea Alastre"`
	Phone    string `json:"phone" example:"+56 9 8765 4321"`
}
