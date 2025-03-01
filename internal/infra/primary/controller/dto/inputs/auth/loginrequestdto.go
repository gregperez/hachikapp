package auth

// LoginRequestDto representa el cuerpo de la petición de login
// @Description Estructura esperada para autenticación de usuario
type LoginRequestDto struct {
	Email    string `json:"email" binding:"required,email" example:"user@example.com"` // 🔹 Ejemplo de correo
	Password string `json:"password" binding:"required" example:"password123"`         // 🔹 Ejemplo de contraseña
}
