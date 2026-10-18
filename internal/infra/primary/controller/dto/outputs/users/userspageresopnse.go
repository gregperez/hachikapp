package userout

// UsersPageResponse UserResponse represents a user response.
// @swagger:model
// @Description UserResponse represents a user response.
type UsersPageResponse struct {
	Limit      int            `json:"limit"`
	HasNext    bool           `json:"hasNext"`
	NextCursor string         `json:"nextCursor,omitempty"`
	Data       []UserResponse `json:"data"`
}
