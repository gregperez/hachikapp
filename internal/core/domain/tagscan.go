package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/netip"
	"strings"
	"time"
)

const (
	// ScanDedupeWindow is how long repeated scans by one visitor of one tag are
	// folded into a single scan row.
	ScanDedupeWindow = 10 * time.Minute

	scanTokenBytes = 32

	// MaxFinderMessageBodyLength and MaxFinderNameLength are counted in runes.
	MaxFinderMessageBodyLength = 500
	MaxFinderNameLength        = 80

	finderFieldBody  = "body"
	finderFieldName  = "finderName"
	finderFieldPhone = "finderPhone"
)

// TagScan is one visit to a public tag page. Only hashes and coarse network
// data are stored, never the raw IP or user agent.
type TagScan struct {
	ID              string
	TagID           string
	PetID           string
	ScannedAt       time.Time
	VisitorHash     string
	ScanTokenHash   string
	IPPrefix        string
	UserAgentFamily string
	PhoneRevealedAt *time.Time
	// Location is what the finder chose to share, nil until they do.
	Location *ScanLocation
}

// TagMessage is a message a finder left for the owner.
type TagMessage struct {
	ID          string
	ScanID      string
	TagID       string
	PetID       string
	Body        string
	FinderName  string
	FinderPhone string
	CreatedAt   time.Time
}

// ScanWithMessages is a scan plus the messages left during it, for the owner's history.
type ScanWithMessages struct {
	TagScan
	TagCode  string
	Messages []TagMessage
}

// FinderMessageInput is the untrusted payload of a finder message.
type FinderMessageInput struct {
	Body        string
	FinderName  string
	FinderPhone string
}

// KeyedHash returns the hex HMAC-SHA256 of value under pepper (64 characters).
func KeyedHash(pepper []byte, value string) string {
	mac := hmac.New(sha256.New, pepper)
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

// NewScanToken returns 32 random bytes from r as unpadded base64url.
func NewScanToken(r io.Reader) (string, error) {
	buf := make([]byte, scanTokenBytes)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// IPPrefix truncates an IP to its /24 (IPv4) or /48 (IPv6) network. An
// IPv4-mapped IPv6 address is treated as IPv4. It returns "" for invalid input.
func IPPrefix(ip string) string {
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return ""
	}
	addr = addr.Unmap().WithZone("")
	bits := 48
	if addr.Is4() {
		bits = 24
	}
	prefix, err := addr.Prefix(bits)
	if err != nil {
		return ""
	}
	return prefix.String()
}

// UserAgentFamily classifies a user agent as ios, android, desktop or other.
func UserAgentFamily(ua string) string {
	ua = strings.ToLower(ua)
	switch {
	case containsAny(ua, "iphone", "ipad", "ipod"):
		return "ios"
	case strings.Contains(ua, "android"):
		return "android"
	case containsAny(ua, "windows", "macintosh", "x11", "linux"):
		return "desktop"
	default:
		return "other"
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// NormalizeFinderMessage trims and collapses whitespace. The body keeps its
// line breaks (CRLF and CR become LF).
func NormalizeFinderMessage(in FinderMessageInput) FinderMessageInput {
	body := strings.ReplaceAll(in.Body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	return FinderMessageInput{
		Body:        strings.TrimSpace(body),
		FinderName:  collapseSpaces(in.FinderName),
		FinderPhone: collapseSpaces(in.FinderPhone),
	}
}

// ValidateFinderMessage checks a normalized message. The body is required; the
// name and phone are optional and the phone follows the pet owner phone rules.
func ValidateFinderMessage(in FinderMessageInput) error {
	var issues []PetFieldIssue
	add := func(field, code string) { issues = append(issues, PetFieldIssue{Field: field, Code: code}) }
	checkText(in.Body, finderFieldBody, MaxFinderMessageBodyLength, true, add)
	checkText(in.FinderName, finderFieldName, MaxFinderNameLength, false, add)
	checkPhone(in.FinderPhone, finderFieldPhone, false, add)
	if len(issues) == 0 {
		return nil
	}
	sortIssues(issues)
	return &PetValidationError{Issues: issues}
}
