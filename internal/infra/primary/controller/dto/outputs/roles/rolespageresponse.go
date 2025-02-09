package roles

type RolesPageResponse struct {
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
	TotalItems int            `json:"totalItems"`
	TotalPages int            `json:"totalPages"`
	Data       []RoleResponse `json:"data"`
}
