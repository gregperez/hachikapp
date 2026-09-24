package dto

import (
	"time"

	"github.com/WilsonSayago/authBase/v3/core/domain"
)

// BaseDto represents a base dto with common fields for all entities
// @swagger:model
// @Description BaseDto contains common fields used across all DTOs in the system
type BaseDto struct {
	Id        string     `json:"id,omitempty"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
	CreatedAt *time.Time `json:"createdAt,omitempty"`
	CreatedBy string     `json:"createdBy,omitempty"`
	UpdatedBy string     `json:"updatedBy,omitempty"`
	Active    bool       `json:"active"`
	Messages  string     `json:"messagesDefault,omitempty"`
}

func (baseDto *BaseDto) DomainToEntity(base domain.Base) {
	baseDto.Id = base.Id
	baseDto.CreatedBy = base.CreatedBy
	baseDto.UpdatedBy = base.UpdatedBy
	baseDto.Active = base.Active
	baseDto.Messages = base.Messages
}
