package userout

// UsersPageResponse UserResponse represents a user response.
// @swagger:model
// @Description UserResponse represents a user response.
type UsersPageResponse struct {
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
	TotalItems int            `json:"totalItems"`
	TotalPages int            `json:"totalPages"`
	Data       []UserResponse `json:"data"`
}
