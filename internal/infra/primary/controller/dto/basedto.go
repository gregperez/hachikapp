package dto

import "time"

type BaseDto struct {
	Id        string     `json:"id,omitempty"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
	CreatedAt *time.Time `json:"createdAt,omitempty"`
	CreatedBy string     `json:"createdBy,omitempty"`
	UpdatedBy string     `json:"updatedBy,omitempty"`
	Active    bool       `json:"active"`
	Messages  string     `json:"messagesDefault,omitempty"`
}
