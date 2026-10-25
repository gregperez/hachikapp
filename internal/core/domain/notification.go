package domain

import (
	"errors"
	"math"
	"time"
)

// NotificationKind identifies why an owner is being notified.
type NotificationKind string

const (
	NotificationTagScanned     NotificationKind = "tag_scanned"
	NotificationLocationShared NotificationKind = "location_shared"
	NotificationFinderMessage  NotificationKind = "finder_message"
	NotificationPhoneRevealed  NotificationKind = "phone_revealed"
)

const (
	// FirstEmailDelay is how long the first email waits so that location,
	// message or contact events of the same scan are grouped in one email.
	FirstEmailDelay = 60 * time.Second
	// DeliveryLease is how long a claimed delivery stays reserved for a worker.
	DeliveryLease = 2 * time.Minute
	// LocationRetention is how long coordinates are kept before being nulled.
	LocationRetention = 30 * 24 * time.Hour
	// ScanRetention is how long scans, messages and notifications are kept.
	ScanRetention = 180 * 24 * time.Hour
)

// ErrLocationAlreadyShared is returned when a scan already has a location (maps to HTTP 409).
var ErrLocationAlreadyShared = errors.New("location already shared for this scan")

// ScanLocation is the position a finder chose to share for a scan.
type ScanLocation struct {
	Lat       float64   `json:"lat"`
	Lng       float64   `json:"lng"`
	AccuracyM int       `json:"accuracyM"`
	At        time.Time `json:"at"`
}

// TagMessageSummary is the finder message as carried inside a notification.
type TagMessageSummary struct {
	Body        string `json:"body"`
	FinderName  string `json:"finderName"`
	FinderPhone string `json:"finderPhone"`
}

// NotificationPayload is the denormalized content of a notification.
type NotificationPayload struct {
	PetName       string             `json:"petName"`
	TagCode       string             `json:"tagCode"`
	ScannedAt     time.Time          `json:"scannedAt"`
	Device        string             `json:"device,omitempty"`
	Location      *ScanLocation      `json:"location,omitempty"`
	Message       *TagMessageSummary `json:"message,omitempty"`
	PhoneRevealed bool               `json:"phoneRevealed,omitempty"`
}

// Notification is an in-app notice for a pet owner.
type Notification struct {
	ID        string
	UserID    string
	PetID     string
	TagID     string
	ScanID    string
	MessageID string
	Kind      NotificationKind
	Payload   NotificationPayload
	ReadAt    *time.Time
	CreatedAt time.Time
}

// NotificationDraft is what a producer asks the store to create.
type NotificationDraft struct {
	UserID  string
	PetID   string
	TagID   string
	Kind    NotificationKind
	Payload NotificationPayload
	// Email asks for an email delivery besides the in-app notice.
	Email bool
}

// Delivery is a claimed email delivery with everything needed to render it.
type Delivery struct {
	ID             string
	NotificationID string
	Attempts       int
	Notification   Notification
	ToEmail        string
	OwnerName      string
}

// Email is a rendered message ready for a Mailer.
type Email struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

var deliveryBackoff = []time.Duration{
	time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour,
}

// NextDeliveryAttempt returns when to retry after a failed send. Callers pass
// the attempt count INCLUDING the one that just failed (Delivery.Attempts+1),
// 1-based. failed is true once the retry ladder is exhausted (6 or more).
func NextDeliveryAttempt(attempts int, now time.Time) (next time.Time, failed bool) {
	if attempts < 1 {
		attempts = 1
	}
	if attempts > len(deliveryBackoff) {
		return time.Time{}, true
	}
	return now.Add(deliveryBackoff[attempts-1]), false
}

// ValidateScanLocation checks coordinate and accuracy ranges. It returns a
// *PetValidationError with field lat, lng or accuracy and code range.
func ValidateScanLocation(lat, lng float64, accuracy int) error {
	var issues []PetFieldIssue
	if math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 {
		issues = append(issues, PetFieldIssue{Field: "lat", Code: "range"})
	}
	if math.IsNaN(lng) || math.IsInf(lng, 0) || lng < -180 || lng > 180 {
		issues = append(issues, PetFieldIssue{Field: "lng", Code: "range"})
	}
	if accuracy < 0 || accuracy > 100000 {
		issues = append(issues, PetFieldIssue{Field: "accuracy", Code: "range"})
	}
	if len(issues) == 0 {
		return nil
	}
	return &PetValidationError{Issues: issues}
}
