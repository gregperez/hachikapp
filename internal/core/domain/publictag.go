package domain

import "errors"

// ErrInvalidScanToken is returned when a scan token is missing or does not
// match the scan. It maps to HTTP 404 so callers cannot tell that the scan exists.
var ErrInvalidScanToken = errors.New("invalid scan token")

// PublicTagView is what anyone who scans a tag sees. Profile is nil unless the
// tag is active.
type PublicTagView struct {
	State   TagStatus
	Profile *PublicProfile
}

// ScanReceipt identifies a recorded scan. Token is the plain bearer secret for
// follow-up actions; only its hash is stored.
type ScanReceipt struct {
	ScanID string
	Token  string
}

// PublicContact is the owner's phone revealed to a finder.
type PublicContact struct {
	Phone       string
	WhatsAppURL string
}
