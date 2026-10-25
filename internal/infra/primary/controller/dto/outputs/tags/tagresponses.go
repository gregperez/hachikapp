package tags

// PublicTagViewResponse is the anonymous view of a tag. profile is present only
// when state is active. It never carries phone numbers, addresses, chip numbers
// or surnames.
// @Description Public tag page. Hidden or empty fields are omitted.
type PublicTagViewResponse struct {
	State   string                 `json:"state" enums:"active,unassigned,disabled" example:"active"`
	Profile *PublicProfileResponse `json:"profile,omitempty"`
}

// PublicProfileResponse holds only the fields the owner made public.
type PublicProfileResponse struct {
	Name                 string   `json:"name" example:"Luna"`
	PhotoURL             string   `json:"photoUrl,omitempty" example:"/v1/public/tags/K7M2Q9XR4P/photo?v=1760820000000"`
	Species              string   `json:"species,omitempty" example:"dog"`
	Breed                string   `json:"breed,omitempty" example:"Mestiza"`
	Color                string   `json:"color,omitempty" example:"Café con blanco"`
	Sex                  string   `json:"sex,omitempty" example:"female"`
	Size                 string   `json:"size,omitempty" example:"medium"`
	AgeMonths            *int     `json:"ageMonths,omitempty" example:"67"`
	DistinctiveFeatures  string   `json:"distinctiveFeatures,omitempty"`
	Allergies            []string `json:"allergies,omitempty"`
	SpecialConditions    []string `json:"specialConditions,omitempty"`
	Surgeries            []string `json:"surgeries,omitempty"`
	Temperament          string   `json:"temperament,omitempty"`
	OwnerFirstName       string   `json:"ownerFirstName,omitempty" example:"Wilson"`
	SecondOwnerFirstName string   `json:"secondOwnerFirstName,omitempty" example:"Andrea"`
	Comuna               string   `json:"comuna,omitempty" example:"Ñuñoa"`
	CanContact           bool     `json:"canContact" example:"true"`
	Lost                 bool     `json:"lost" example:"false"`
}

// ScanReceiptResponse identifies a scan and the token that authorizes follow-up calls.
type ScanReceiptResponse struct {
	ScanID    string `json:"scanId" example:"3c0b7f8e-1d2e-4f5a-9b6c-7d8e9f0a1b2c"`
	ScanToken string `json:"scanToken" example:"q0bW9z..."`
}

// PublicContactResponse is the owner's main phone, revealed on demand.
type PublicContactResponse struct {
	Phone       string `json:"phone" example:"+56 9 1234 5678"`
	WhatsAppURL string `json:"whatsappUrl" example:"https://wa.me/56912345678"`
}

// TagResponse is a tag as seen by its owner or an admin.
type TagResponse struct {
	ID          string `json:"id" example:"7a9e2c1d-0b3f-4e5d-8c7b-6a5f4e3d2c1b"`
	Code        string `json:"code" example:"K7M2Q9XR4P"`
	Kind        string `json:"kind" enums:"physical,digital" example:"digital"`
	Status      string `json:"status" enums:"unassigned,active,disabled" example:"active"`
	PetID       string `json:"petId,omitempty" example:"2f1c5a0e-6a43-4f8e-9a51-0c2f3f9b7d11"`
	ActivatedAt string `json:"activatedAt,omitempty" example:"2026-10-18T21:00:00Z"`
	DisabledAt  string `json:"disabledAt,omitempty"`
	CreatedAt   string `json:"createdAt" example:"2026-10-18T21:00:00Z"`
	URL         string `json:"url" example:"https://hachikapp.cl/t/K7M2Q9XR4P"`
}

// ScanMessageResponse is a message a finder left on a scan.
type ScanMessageResponse struct {
	ID          string `json:"id"`
	Body        string `json:"body" example:"Encontré a Luna en la plaza."`
	FinderName  string `json:"finderName,omitempty" example:"Camila"`
	FinderPhone string `json:"finderPhone,omitempty" example:"+56 9 1111 2222"`
	CreatedAt   string `json:"createdAt" example:"2026-10-18T21:05:00Z"`
}

// ScanLocationResponse is the position a finder shared for a scan.
type ScanLocationResponse struct {
	Lat       float64 `json:"lat" example:"-33.4372"`
	Lng       float64 `json:"lng" example:"-70.6506"`
	AccuracyM int     `json:"accuracyM" example:"35"`
	At        string  `json:"at" example:"2026-10-18T21:02:00Z"`
}

// ScanResponse is a scan of one of the owner's tags.
type ScanResponse struct {
	ID              string                `json:"id"`
	TagCode         string                `json:"tagCode" example:"K7M2Q9XR4P"`
	ScannedAt       string                `json:"scannedAt" example:"2026-10-18T21:00:00Z"`
	UserAgentFamily string                `json:"userAgentFamily" enums:"ios,android,desktop,other" example:"ios"`
	PhoneRevealedAt string                `json:"phoneRevealedAt,omitempty"`
	Location        *ScanLocationResponse `json:"location,omitempty"`
	Messages        []ScanMessageResponse `json:"messages"`
}

// ScansPageResponse is a keyset page of scans.
type ScansPageResponse struct {
	Limit      int            `json:"limit"`
	HasNext    bool           `json:"hasNext"`
	NextCursor string         `json:"nextCursor,omitempty"`
	Data       []ScanResponse `json:"data"`
}

// TagBatchResponse is a generated batch of physical tags.
type TagBatchResponse struct {
	ID        string `json:"id" example:"5b6c7d8e-9f0a-4b1c-8d2e-3f4a5b6c7d8e"`
	Label     string `json:"label" example:"Lote octubre 2026"`
	Quantity  int    `json:"quantity" example:"100"`
	CreatedAt string `json:"createdAt" example:"2026-10-18T21:00:00Z"`
}

// TagBatchesPageResponse is a keyset page of batches.
type TagBatchesPageResponse struct {
	Limit      int                `json:"limit"`
	HasNext    bool               `json:"hasNext"`
	NextCursor string             `json:"nextCursor,omitempty"`
	Data       []TagBatchResponse `json:"data"`
}
