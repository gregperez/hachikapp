package dto

import "time"

type BaseDTO struct {
	Id        string    `bson:"id,omitempty"`
	UpdatedAt time.Time `bson:"updatedAt,omitempty"`
	CreatedAt time.Time `bson:"createdAt,omitempty"`
	CreatedBy string    `bson:"createdBy,omitempty"`
	UpdatedBy string    `bson:"updatedBy,omitempty"`
	Active    bool      `bson:"active,omitempty"`
	Messages  string    `bson:"messagesDefault,omitempty"`
}
