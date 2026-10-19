package tags

// ScanRequest is the body of POST /v1/public/tags/{code}/scans.
// @Description Anonymous scan registration. visitorId is a UUID kept by the browser.
type ScanRequest struct {
	VisitorID string `json:"visitorId" example:"6f1d2a8e-3b4c-4d5e-8f90-1a2b3c4d5e6f"`
}

// FinderMessageRequest is the body of POST /v1/public/scans/{scanId}/messages.
// @Description Message left by whoever found the pet. website is a honeypot and must stay empty.
type FinderMessageRequest struct {
	Body        string `json:"body" example:"Encontré a Luna en la plaza, está bien."`
	FinderName  string `json:"finderName,omitempty" example:"Camila"`
	FinderPhone string `json:"finderPhone,omitempty" example:"+56 9 1111 2222"`
	Website     string `json:"website,omitempty" example:""`
}

// ActivateTagRequest is the body of POST /v1/tags/{code}/activate.
type ActivateTagRequest struct {
	PetID string `json:"petId" example:"2f1c5a0e-6a43-4f8e-9a51-0c2f3f9b7d11"`
}

// CreateTagBatchRequest is the body of POST /v1/admin/tag-batches.
// @Description Batch of physical tags to generate (1 to 1000 codes).
type CreateTagBatchRequest struct {
	Label    string `json:"label" example:"Lote octubre 2026"`
	Quantity int    `json:"quantity" example:"100"`
}
