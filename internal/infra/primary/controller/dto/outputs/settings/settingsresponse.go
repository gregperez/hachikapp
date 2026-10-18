package settings

// PublicSettingsResponse is the unauthenticated branding subset.
// @Description Public branding fields available before and after login.
type PublicSettingsResponse struct {
	DisplayName   string `json:"displayName"`
	LogoURL       string `json:"logoUrl"`
	DefaultLocale string `json:"defaultLocale"`
}

// SettingsResponse is the administrative view of application settings.
// @Description Typed application settings including optimistic version.
type SettingsResponse struct {
	DisplayName   string `json:"displayName"`
	LogoURL       string `json:"logoUrl"`
	DefaultLocale string `json:"defaultLocale"`
	Version       int64  `json:"version"`
	UpdatedAt     string `json:"updatedAt,omitempty"`
	UpdatedBy     string `json:"updatedBy,omitempty"`
}

// UpdateSettingsRequest is the closed writable payload.
// @Description Allowlisted application settings. Unknown or protected keys are rejected.
type UpdateSettingsRequest struct {
	Version       int64  `json:"version"`
	DisplayName   string `json:"displayName"`
	LogoURL       string `json:"logoUrl"`
	DefaultLocale string `json:"defaultLocale"`
}
