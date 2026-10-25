package notifications

import "github.com/gregperez/hachikapp-backend/internal/core/domain"

// NotificationResponse is one in-app notification of the owner.
type NotificationResponse struct {
	ID        string                     `json:"id" example:"9c2f3a1e-5b7d-4c8e-9a10-2b3c4d5e6f70"`
	Kind      string                     `json:"kind" enums:"tag_scanned,location_shared,finder_message,phone_revealed" example:"tag_scanned"`
	PetID     string                     `json:"petId"`
	TagID     string                     `json:"tagId"`
	ScanID    string                     `json:"scanId,omitempty"`
	Payload   domain.NotificationPayload `json:"payload"`
	ReadAt    string                     `json:"readAt,omitempty" example:"2026-10-25T10:00:00Z"`
	CreatedAt string                     `json:"createdAt" example:"2026-10-25T09:00:00Z"`
}

// NotificationsPageResponse is a keyset page of notifications, newest first.
type NotificationsPageResponse struct {
	Limit      int                    `json:"limit"`
	HasNext    bool                   `json:"hasNext"`
	NextCursor string                 `json:"nextCursor,omitempty"`
	Data       []NotificationResponse `json:"data"`
}

// UnreadCountResponse is the number of unread notifications.
type UnreadCountResponse struct {
	Count int `json:"count" example:"3"`
}

// ReadAllResponse is how many notifications were marked as read.
type ReadAllResponse struct {
	Updated int `json:"updated" example:"3"`
}
