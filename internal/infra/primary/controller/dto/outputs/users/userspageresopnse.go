package users

type UsersPageResponse struct {
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
	TotalItems int            `json:"totalItems"`
	TotalPages int            `json:"totalPages"`
	Data       []UserResponse `json:"data"`
}
