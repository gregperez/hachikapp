package dto

import (
	"github.com/WilsonSayago/authBase/core/domain"
	"time"
)

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
	baseDto.UpdatedAt = &base.UpdatedAt
	baseDto.CreatedAt = &base.CreatedAt
	baseDto.CreatedBy = base.CreatedBy
	baseDto.UpdatedBy = base.UpdatedBy
	baseDto.Active = base.Active
	baseDto.Messages = base.Messages
}
