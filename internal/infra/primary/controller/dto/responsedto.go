package dto

// ResponseDto represents the standard response structure for all API endpoints
// @swagger:model
// @Description Standard response model used across all API endpoints
type ResponseDto struct {
	Status  int         `json:"status" example:"200"` // 🔹 Código HTTP de respuesta
	Message string      `json:"message" example:"Ok"` // 🔹 Mensaje descriptivo de la respuesta
	Data    interface{} `json:"data"`                 // 🔹 Datos dinámicos según el endpoint
}
