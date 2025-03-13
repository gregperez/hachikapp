package dto

// AuthDto represents the authentication response containing access and refresh tokens
// @swagger:model
// @Description Authentication response model containing JWT tokens for user authentication
type AuthDto struct {
	AccessToken  string `json:"accessToken" example:"eyJhbGciOiJIUzI1..."` // 🔹 Token JWT de acceso
	RefreshToken string `json:"refreshToken" example:"eyJhbGciOiJIUzI1..."`
}
