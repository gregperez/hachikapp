package dto

// AuthDto representa la respuesta de autenticación
// @Description Contiene los tokens generados tras una autenticación exitosa
type AuthDto struct {
	AccessToken  string `json:"accessToken" example:"eyJhbGciOiJIUzI1..."` // 🔹 Token JWT de acceso
	RefreshToken string `json:"refreshToken" example:"eyJhbGciOiJIUzI1..."`
}
