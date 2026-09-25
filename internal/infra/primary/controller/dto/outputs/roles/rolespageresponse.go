package roles

// RolesPageResponse represents a paginated response of roles.
// @swagger:model
// @Description RolesPageResponse represents a paginated response of roles.
type RolesPageResponse struct {
	Limit      int            `json:"limit"`
	HasNext    bool           `json:"hasNext"`
	NextCursor string         `json:"nextCursor,omitempty"`
	Data       []RoleResponse `json:"data"`
}
