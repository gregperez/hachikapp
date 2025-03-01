package roles

// RolesPageResponse represents a paginated response of roles.
// @swagger:model
// @Description RolesPageResponse represents a paginated response of roles.
type RolesPageResponse struct {
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
	TotalItems int            `json:"totalItems"`
	TotalPages int            `json:"totalPages"`
	Data       []RoleResponse `json:"data"`
}
