package auth

// RefreshTokenRequestDto representa el cuerpo de la petición de refresco de token
// @Description Estructura esperada para refrescar un token JWT
type RefreshTokenRequestDto struct {
	RefreshToken string `json:"refreshToken" binding:"required" example:"eyJhbGciOiJIUzI1..."` // 🔹 Token de refresco
}
