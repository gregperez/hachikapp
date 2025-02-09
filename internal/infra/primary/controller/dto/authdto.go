package dto

type AuthDto struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
}
