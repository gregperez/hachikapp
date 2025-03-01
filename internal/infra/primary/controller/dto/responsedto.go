package dto

// ResponseDto representa la estructura de respuestas genéricas
// @Description Modelo de respuesta estándar para los endpoints
type ResponseDto struct {
	Status  int         `json:"status" example:"200"` // 🔹 Código HTTP de respuesta
	Message string      `json:"message" example:"Ok"` // 🔹 Mensaje descriptivo de la respuesta
	Data    interface{} `json:"data"`                 // 🔹 Datos dinámicos según el endpoint
}
