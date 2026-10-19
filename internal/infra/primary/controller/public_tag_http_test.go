package controller

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gin-gonic/gin"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/core/services"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/httpapi"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/ratelimit"
)

const (
	publicTestCode    = "K7M2Q9XR4P"
	publicTestTagID   = "7a9e2c1d-0b3f-4e5d-8c7b-6a5f4e3d2c1b"
	publicTestOwnerID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	publicTestVisitor = "6f1d2a8e-3b4c-4d5e-8f90-1a2b3c4d5e6f"
	publicTestPepper  = "controller-test-pepper"
)

// privateNeedles are values that must never appear in a public response other
// than /contact (which returns only the main phone).
var privateNeedles = []string{
	"1234 5678", "912345678", // owner phone
	"8765 4321",                // owner alt phone
	"5555 4444",                // second owner phone
	"Siempre Viva",             // address
	"900123456789012",          // chip
	"Sayago", "Pérez", "Rojas", // surnames
}

// --- in-memory ports so the HTTP tests exercise the real PublicTagService ---

type memTags struct{ tags map[string]domain.PetTag }

func (m *memTags) CreateBatch(context.Context, domain.TagBatch, []string) (domain.TagBatch, error) {
	panic("unused")
}
func (m *memTags) ListBatches(context.Context, authBaseDomain.PageRequest) (authBaseDomain.Page[domain.TagBatch], error) {
	panic("unused")
}
func (m *memTags) BatchTags(context.Context, string) ([]domain.PetTag, error) { panic("unused") }
func (m *memTags) GetByCode(_ context.Context, code string) (domain.PetTag, error) {
	for _, tag := range m.tags {
		if tag.Code == code {
			return tag, nil
		}
	}
	return domain.PetTag{}, domain.ErrNotFound
}
func (m *memTags) GetByID(_ context.Context, id string) (domain.PetTag, error) {
	tag, ok := m.tags[id]
	if !ok {
		return domain.PetTag{}, domain.ErrNotFound
	}
	return tag, nil
}
func (m *memTags) ListByPet(context.Context, string, string) ([]domain.PetTag, error) {
	panic("unused")
}
func (m *memTags) Activate(context.Context, string, string, string, int, time.Time) (domain.PetTag, error) {
	panic("unused")
}
func (m *memTags) CreateDigital(context.Context, string, string, string, int, time.Time) (domain.PetTag, error) {
	panic("unused")
}
func (m *memTags) Disable(context.Context, string, string, string, time.Time) (domain.PetTag, error) {
	panic("unused")
}

type memScans struct {
	mu       sync.Mutex
	scans    map[string]domain.TagScan
	messages []domain.TagMessage
	revealed int
}

func (m *memScans) RecordScan(_ context.Context, scan domain.TagScan, _ time.Time) (domain.TagScan, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	scan.ID = fmt.Sprintf("33333333-3333-3333-3333-%012d", len(m.scans)+1)
	m.scans[scan.ID] = scan
	return scan, false, nil
}
func (m *memScans) GetScan(_ context.Context, id string) (domain.TagScan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	scan, ok := m.scans[id]
	if !ok {
		return domain.TagScan{}, domain.ErrNotFound
	}
	return scan, nil
}
func (m *memScans) AddMessage(_ context.Context, msg domain.TagMessage) (domain.TagMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
	return msg, nil
}
func (m *memScans) MarkPhoneRevealed(context.Context, string, time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revealed++
	return m.revealed == 1, nil
}
func (m *memScans) ListScansByPet(context.Context, string, string, authBaseDomain.PageRequest) (authBaseDomain.Page[domain.ScanWithMessages], error) {
	panic("unused")
}

type memPets struct{ pet domain.Pet }

func (m *memPets) ListByOwner(context.Context, string, int) ([]domain.Pet, error) { panic("unused") }
func (m *memPets) CountByOwner(context.Context, string) (int, error)              { panic("unused") }
func (m *memPets) GetForOwner(_ context.Context, ownerID, petID string) (domain.Pet, error) {
	if ownerID != m.pet.OwnerID || petID != m.pet.ID {
		return domain.Pet{}, domain.ErrNotFound
	}
	return m.pet, nil
}
func (m *memPets) Create(context.Context, string, domain.PetDetails) (domain.Pet, error) {
	panic("unused")
}
func (m *memPets) Update(context.Context, string, string, int64, domain.PetDetails) (domain.Pet, error) {
	panic("unused")
}
func (m *memPets) SetPhotoUpdatedAt(context.Context, string, string, *time.Time) (domain.Pet, error) {
	panic("unused")
}
func (m *memPets) Delete(context.Context, string, string) error { panic("unused") }

type memPhotos struct{}

func (memPhotos) Get(context.Context, string) (domain.PetPhoto, error) {
	return domain.PetPhoto{ContentType: "image/png", Data: []byte("png-bytes")}, nil
}
func (memPhotos) Put(context.Context, string, domain.PetPhoto) error { panic("unused") }
func (memPhotos) Delete(context.Context, string) error               { panic("unused") }

// --- fixture ---

func publicTestPet(visibility domain.PetVisibility) domain.Pet {
	birth := time.Date(2021, 3, 14, 0, 0, 0, 0, time.UTC)
	all := map[domain.PetPublicField]domain.PetVisibility{}
	for _, field := range []domain.PetPublicField{
		domain.PetPublicPhoto, domain.PetPublicSpecies, domain.PetPublicBreed, domain.PetPublicColor,
		domain.PetPublicSex, domain.PetPublicAge, domain.PetPublicSize, domain.PetPublicDistinctiveFeatures,
		domain.PetPublicAllergies, domain.PetPublicSpecialConditions, domain.PetPublicSurgeries,
		domain.PetPublicTemperament, domain.PetPublicOwnerName, domain.PetPublicContactButton,
		domain.PetPublicComuna, domain.PetPublicSecondOwner,
	} {
		all[field] = visibility
	}
	return domain.Pet{
		ID:      testPetID,
		OwnerID: publicTestOwnerID,
		PetDetails: domain.PetDetails{
			Name: "Luna", Species: "dog", Breed: "Mestiza", Color: "Café", Sex: "female",
			BirthDate: &birth, ChipNumber: "900123456789012", Size: "medium",
			DistinctiveFeatures: "Mancha blanca",
			Health:              domain.PetHealth{Allergies: []string{"Atún"}, SpecialConditions: []string{"Sorda"}, Surgeries: []string{"Esterilizada"}, Temperament: "Tímida"},
			Owner:               domain.PetOwnerContact{FullName: "Wilson Sayago Pérez", Address: "Av. Siempre Viva 742", Comuna: "Ñuñoa", Phone: "+56 9 1234 5678", PhoneAlt: "+56 9 8765 4321"},
			SecondOwner:         domain.PetSecondOwner{FullName: "Andrea Rojas", Phone: "+56 9 5555 4444"},
			PublicVisibility:    all,
		},
		HasPhoto:       true,
		PhotoUpdatedAt: time.UnixMilli(1760820000000).UTC(),
	}
}

type publicFixture struct {
	engine *gin.Engine
	tags   *memTags
	scans  *memScans
	pets   *memPets
}

func newPublicFixture(t *testing.T, pet domain.Pet, limiter ports.LoginLimiter) *publicFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	f := &publicFixture{
		tags: &memTags{tags: map[string]domain.PetTag{publicTestTagID: {
			ID: publicTestTagID, Code: publicTestCode, Kind: domain.TagKindPhysical, Status: domain.TagStatusActive,
			PetID: pet.ID, OwnerID: pet.OwnerID,
		}}},
		scans: &memScans{scans: map[string]domain.TagScan{}},
		pets:  &memPets{pet: pet},
	}
	uc := services.NewPublicTagService(f.tags, f.scans, f.pets, memPhotos{}, []byte(publicTestPepper), rand.Reader)
	if limiter == nil {
		limiter = newPublicLimiter(t, ratelimit.PublicTagScopes())
	}
	resolver, err := NewClientOriginResolver(nil)
	if err != nil {
		t.Fatal(err)
	}
	f.engine = gin.New()
	f.engine.Use(httpapi.LimitRequestBody(httpapi.DefaultMaxBodyBytes))
	NewPublicTagController(uc, limiter, resolver, []byte(publicTestPepper), f.engine).RunController()
	return f
}

func newPublicLimiter(t *testing.T, scopes map[ports.LoginLimitScope]ratelimit.ScopeConfig) ports.LoginLimiter {
	t.Helper()
	limiter, err := ratelimit.NewMemory(ratelimit.Config{Scopes: scopes, MaxEntries: 1000})
	if err != nil {
		t.Fatal(err)
	}
	return limiter
}

func (f *publicFixture) do(method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	return f.doFrom("192.0.2.10:4000", method, path, body, headers)
}

func (f *publicFixture) doFrom(remote, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = remote
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	f.engine.ServeHTTP(w, req)
	return w
}

func (f *publicFixture) scan(t *testing.T) (string, string) {
	t.Helper()
	w := f.do(http.MethodPost, "/v1/public/tags/"+publicTestCode+"/scans", `{"visitorId":"`+publicTestVisitor+`"}`, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("scan status=%d body=%s", w.Code, w.Body.String())
	}
	var env struct {
		Data struct {
			ScanID    string `json:"scanId"`
			ScanToken string `json:"scanToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.ScanID == "" || env.Data.ScanToken == "" {
		t.Fatalf("scan receipt=%s", w.Body.String())
	}
	return env.Data.ScanID, env.Data.ScanToken
}

func assertNoPrivateData(t *testing.T, label string, w *httptest.ResponseRecorder) {
	t.Helper()
	body := w.Body.String()
	for _, needle := range privateNeedles {
		if strings.Contains(body, needle) {
			t.Fatalf("%s leaks %q: %s", label, needle, body)
		}
	}
	for _, header := range w.Header() {
		for _, value := range header {
			for _, needle := range privateNeedles {
				if strings.Contains(value, needle) {
					t.Fatalf("%s header leaks %q", label, needle)
				}
			}
		}
	}
}

// --- tests ---

func TestPublicTagResponseNeverLeaksPrivateData(t *testing.T) {
	f := newPublicFixture(t, publicTestPet(domain.PetVisibilityAlways), nil)

	profile := f.do(http.MethodGet, "/v1/public/tags/"+publicTestCode, "", nil)
	if profile.Code != http.StatusOK {
		t.Fatalf("profile status=%d body=%s", profile.Code, profile.Body.String())
	}
	assertNoPrivateData(t, "profile", profile)
	if !strings.Contains(profile.Body.String(), `"ownerFirstName":"Wilson"`) || !strings.Contains(profile.Body.String(), `"secondOwnerFirstName":"Andrea"`) {
		t.Fatalf("profile should show first names only: %s", profile.Body.String())
	}

	scanID, token := f.scan(t)
	responses := map[string]*httptest.ResponseRecorder{
		"photo":            f.do(http.MethodGet, "/v1/public/tags/"+publicTestCode+"/photo", "", nil),
		"scan":             f.do(http.MethodPost, "/v1/public/tags/"+publicTestCode+"/scans", `{"visitorId":"`+publicTestVisitor+`"}`, nil),
		"scan invalid":     f.do(http.MethodPost, "/v1/public/tags/"+publicTestCode+"/scans", `{"visitorId":"nope"}`, nil),
		"unknown tag":      f.do(http.MethodGet, "/v1/public/tags/ZZZZZZZZZZ", "", nil),
		"malformed tag":    f.do(http.MethodGet, "/v1/public/tags/abc", "", nil),
		"message":          f.do(http.MethodPost, "/v1/public/scans/"+scanID+"/messages", `{"body":"La encontré"}`, map[string]string{"X-Scan-Token": token}),
		"message invalid":  f.do(http.MethodPost, "/v1/public/scans/"+scanID+"/messages", `{"body":""}`, map[string]string{"X-Scan-Token": token}),
		"message badtoken": f.do(http.MethodPost, "/v1/public/scans/"+scanID+"/messages", `{"body":"hola"}`, map[string]string{"X-Scan-Token": "wrong"}),
		"contact badtoken": f.do(http.MethodPost, "/v1/public/scans/"+scanID+"/contact", "", map[string]string{"X-Scan-Token": "wrong"}),
	}
	for label, w := range responses {
		if w.Code >= 500 {
			t.Fatalf("%s status=%d body=%s", label, w.Code, w.Body.String())
		}
		assertNoPrivateData(t, label, w)
	}

	// /contact is the only public route allowed to return the main phone, and
	// never the alternative or second owner phone, address, chip or surname.
	contact := f.do(http.MethodPost, "/v1/public/scans/"+scanID+"/contact", "", map[string]string{"X-Scan-Token": token})
	if contact.Code != http.StatusOK {
		t.Fatalf("contact status=%d body=%s", contact.Code, contact.Body.String())
	}
	if !strings.Contains(contact.Body.String(), `"phone":"+56 9 1234 5678"`) || !strings.Contains(contact.Body.String(), `"whatsappUrl":"https://wa.me/56912345678"`) {
		t.Fatalf("contact body=%s", contact.Body.String())
	}
	for _, needle := range []string{"8765 4321", "5555 4444", "Siempre Viva", "900123456789012", "Sayago", "Rojas"} {
		if strings.Contains(contact.Body.String(), needle) {
			t.Fatalf("contact leaks %q: %s", needle, contact.Body.String())
		}
	}
}

func TestPublicTagProfileShape(t *testing.T) {
	f := newPublicFixture(t, publicTestPet(domain.PetVisibilityAlways), nil)
	// Lowercase and hyphenated input is normalized; the photo URL uses the canonical code.
	w := f.do(http.MethodGet, "/v1/public/tags/k7m2-q9xr4p", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data["state"] != "active" {
		t.Fatalf("state=%v", env.Data["state"])
	}
	profile, _ := env.Data["profile"].(map[string]any)
	want := map[string]any{
		"name": "Luna", "photoUrl": "/v1/public/tags/" + publicTestCode + "/photo?v=1760820000000",
		"species": "dog", "breed": "Mestiza", "color": "Café", "sex": "female", "size": "medium",
		"distinctiveFeatures": "Mancha blanca", "temperament": "Tímida", "ownerFirstName": "Wilson",
		"secondOwnerFirstName": "Andrea", "comuna": "Ñuñoa", "canContact": true, "lost": false,
	}
	for key, value := range want {
		if profile[key] != value {
			t.Fatalf("profile[%s]=%v want %v (%s)", key, profile[key], value, w.Body.String())
		}
	}
	if _, ok := profile["ageMonths"].(float64); !ok {
		t.Fatalf("ageMonths missing: %s", w.Body.String())
	}
	for _, key := range []string{"allergies", "specialConditions", "surgeries"} {
		if _, ok := profile[key].([]any); !ok {
			t.Fatalf("%s missing: %s", key, w.Body.String())
		}
	}

	hidden := newPublicFixture(t, publicTestPet(domain.PetVisibilityHidden), nil)
	w = hidden.do(http.MethodGet, "/v1/public/tags/"+publicTestCode, "", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	profile, _ = env.Data["profile"].(map[string]any)
	if len(profile) != 3 || profile["name"] != "Luna" || profile["canContact"] != false || profile["lost"] != false {
		t.Fatalf("hidden profile should only have name, canContact and lost: %s", w.Body.String())
	}
	if photo := hidden.do(http.MethodGet, "/v1/public/tags/"+publicTestCode+"/photo", "", nil); photo.Code != http.StatusNotFound {
		t.Fatalf("hidden photo status=%d", photo.Code)
	}

	unassigned := newPublicFixture(t, publicTestPet(domain.PetVisibilityAlways), nil)
	tag := unassigned.tags.tags[publicTestTagID]
	tag.Status, tag.PetID, tag.OwnerID = domain.TagStatusUnassigned, "", ""
	unassigned.tags.tags[publicTestTagID] = tag
	w = unassigned.do(http.MethodGet, "/v1/public/tags/"+publicTestCode, "", nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"data":{"state":"unassigned"}`) {
		t.Fatalf("unassigned status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestPublicTagHeaders(t *testing.T) {
	scopes := ratelimit.PublicTagScopes()
	scopes[ports.PublicTagViewLimit] = ratelimit.ScopeConfig{Limit: 3, Window: time.Minute}
	f := newPublicFixture(t, publicTestPet(domain.PetVisibilityAlways), newPublicLimiter(t, scopes))
	scanID, token := f.scan(t)

	cases := map[string]*httptest.ResponseRecorder{
		"profile":   f.do(http.MethodGet, "/v1/public/tags/"+publicTestCode, "", nil),
		"photo":     f.do(http.MethodGet, "/v1/public/tags/"+publicTestCode+"/photo", "", nil),
		"not found": f.do(http.MethodGet, "/v1/public/tags/ZZZZZZZZZZ", "", nil),
		"429":       f.do(http.MethodGet, "/v1/public/tags/"+publicTestCode, "", nil),
		"scan 422":  f.do(http.MethodPost, "/v1/public/tags/"+publicTestCode+"/scans", `{"visitorId":"x"}`, nil),
		"message":   f.do(http.MethodPost, "/v1/public/scans/"+scanID+"/messages", `{"body":"hola"}`, map[string]string{"X-Scan-Token": token}),
		"contact":   f.do(http.MethodPost, "/v1/public/scans/"+scanID+"/contact", "", map[string]string{"X-Scan-Token": token}),
		"bad json":  f.do(http.MethodPost, "/v1/public/tags/"+publicTestCode+"/scans", `{`, nil),
	}
	if cases["429"].Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", cases["429"].Code)
	}
	if got := cases["photo"].Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("photo content-type=%q", got)
	}
	for label, w := range cases {
		if got := w.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("%s Cache-Control=%q", label, got)
		}
		if got := w.Header().Get("X-Robots-Tag"); got != "noindex" {
			t.Fatalf("%s X-Robots-Tag=%q", label, got)
		}
	}
	if problem := decodeProblem(t, cases["not found"]); problem.Status != http.StatusNotFound || problem.Detail != "Tag not found" {
		t.Fatalf("not found problem=%+v", problem)
	}
}

func TestPublicRoutesRateLimited(t *testing.T) {
	f := newPublicFixture(t, publicTestPet(domain.PetVisibilityAlways), nil)
	scanID, token := f.scan(t)
	contact := func(remote string) *httptest.ResponseRecorder {
		return f.doFrom(remote, http.MethodPost, "/v1/public/scans/"+scanID+"/contact", "", map[string]string{"X-Scan-Token": token})
	}
	// 3 contact reveals per hour and IP: the call after the third is refused.
	for i := 0; i < 3; i++ {
		if w := contact("192.0.2.10:4000"); w.Code != http.StatusOK {
			t.Fatalf("contact %d status=%d body=%s", i+1, w.Code, w.Body.String())
		}
	}
	revealed := f.scans.revealed
	w := contact("192.0.2.10:5000")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("over-limit contact status=%d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
	if f.scans.revealed != revealed {
		t.Fatal("rate-limited request reached the use case")
	}
	if w := contact("198.51.100.7:4000"); w.Code != http.StatusOK {
		t.Fatalf("another IP status=%d", w.Code)
	}

	// Every public route is limited by its own scope, before any work.
	tight := map[ports.LoginLimitScope]ratelimit.ScopeConfig{}
	for scope := range ratelimit.PublicTagScopes() {
		tight[scope] = ratelimit.ScopeConfig{Limit: 1, Window: time.Hour}
	}
	g := newPublicFixture(t, publicTestPet(domain.PetVisibilityAlways), newPublicLimiter(t, tight))
	gScan, gToken := g.scan(t)
	routes := []struct {
		name, method, path, body string
	}{
		{"scan", http.MethodPost, "/v1/public/tags/" + publicTestCode + "/scans", `{"visitorId":"` + publicTestVisitor + `"}`},
		{"profile", http.MethodGet, "/v1/public/tags/" + publicTestCode, ""},
		{"photo", http.MethodGet, "/v1/public/tags/" + publicTestCode + "/photo", ""},
		{"message", http.MethodPost, "/v1/public/scans/" + gScan + "/messages", `{"body":"hola"}`},
		{"message", http.MethodPost, "/v1/public/scans/" + gScan + "/messages", `{"body":"hola"}`},
		{"contact", http.MethodPost, "/v1/public/scans/" + gScan + "/contact", ""},
		{"contact", http.MethodPost, "/v1/public/scans/" + gScan + "/contact", ""},
	}
	headers := map[string]string{"X-Scan-Token": gToken}
	// The first scan already used the scan quota; profile and photo share the view quota.
	want := []int{http.StatusTooManyRequests, http.StatusOK, http.StatusTooManyRequests, http.StatusAccepted, http.StatusTooManyRequests, http.StatusOK, http.StatusTooManyRequests}
	for i, route := range routes {
		w := g.do(route.method, route.path, route.body, headers)
		if w.Code != want[i] {
			t.Fatalf("%s #%d status=%d want %d body=%s", route.name, i, w.Code, want[i], w.Body.String())
		}
		if w.Code == http.StatusTooManyRequests && w.Header().Get("Retry-After") == "" {
			t.Fatalf("%s missing Retry-After", route.name)
		}
	}
	if len(g.scans.messages) != 1 || len(g.scans.scans) != 1 {
		t.Fatalf("limited requests reached storage: messages=%d scans=%d", len(g.scans.messages), len(g.scans.scans))
	}
}

func TestHoneypotMessageIsDropped(t *testing.T) {
	scopes := ratelimit.PublicTagScopes()
	scopes[ports.PublicTagMessageLimit] = ratelimit.ScopeConfig{Limit: 100, Window: time.Hour}
	f := newPublicFixture(t, publicTestPet(domain.PetVisibilityAlways), newPublicLimiter(t, scopes))
	scanID, token := f.scan(t)

	for _, headers := range []map[string]string{{"X-Scan-Token": token}, {"X-Scan-Token": "wrong"}} {
		w := f.do(http.MethodPost, "/v1/public/scans/"+scanID+"/messages", `{"body":"compra ya","website":"http://spam.example"}`, headers)
		if w.Code != http.StatusAccepted {
			t.Fatalf("honeypot status=%d body=%s", w.Code, w.Body.String())
		}
	}
	if len(f.scans.messages) != 0 {
		t.Fatalf("honeypot message stored: %+v", f.scans.messages)
	}

	ok := f.do(http.MethodPost, "/v1/public/scans/"+scanID+"/messages", `{"body":"  La encontré en la plaza ","finderName":"Camila","website":"   "}`, map[string]string{"X-Scan-Token": token})
	if ok.Code != http.StatusAccepted {
		t.Fatalf("message status=%d body=%s", ok.Code, ok.Body.String())
	}
	if len(f.scans.messages) != 1 || f.scans.messages[0].Body != "La encontré en la plaza" || f.scans.messages[0].FinderName != "Camila" {
		t.Fatalf("stored=%+v", f.scans.messages)
	}

	invalid := f.do(http.MethodPost, "/v1/public/scans/"+scanID+"/messages", `{"body":"","finderPhone":"abc"}`, map[string]string{"X-Scan-Token": token})
	if invalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid status=%d body=%s", invalid.Code, invalid.Body.String())
	}
	problem := decodeProblem(t, invalid)
	fields := map[string]bool{}
	for _, issue := range problem.Errors {
		fields[issue.Field] = true
	}
	if !fields["body"] || !fields["finderPhone"] {
		t.Fatalf("issues=%+v", problem.Errors)
	}

	badToken := f.do(http.MethodPost, "/v1/public/scans/"+scanID+"/messages", `{"body":"hola"}`, map[string]string{"X-Scan-Token": "wrong"})
	unknown := f.do(http.MethodPost, "/v1/public/scans/44444444-4444-4444-4444-444444444444/messages", `{"body":"hola"}`, map[string]string{"X-Scan-Token": token})
	for label, w := range map[string]*httptest.ResponseRecorder{"bad token": badToken, "unknown scan": unknown} {
		if w.Code != http.StatusNotFound || decodeProblem(t, w).Detail != "Tag not found" {
			t.Fatalf("%s status=%d body=%s", label, w.Code, w.Body.String())
		}
	}
}

func TestContactHiddenReturns404(t *testing.T) {
	pet := publicTestPet(domain.PetVisibilityAlways)
	pet.PublicVisibility[domain.PetPublicContactButton] = domain.PetVisibilityHidden
	f := newPublicFixture(t, pet, nil)
	scanID, token := f.scan(t)

	hidden := f.do(http.MethodPost, "/v1/public/scans/"+scanID+"/contact", "", map[string]string{"X-Scan-Token": token})
	badToken := f.do(http.MethodPost, "/v1/public/scans/"+scanID+"/contact", "", map[string]string{"X-Scan-Token": "wrong"})
	noToken := f.do(http.MethodPost, "/v1/public/scans/"+scanID+"/contact", "", nil)
	for label, w := range map[string]*httptest.ResponseRecorder{"hidden": hidden, "bad token": badToken, "no token": noToken} {
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s status=%d body=%s", label, w.Code, w.Body.String())
		}
		if problem := decodeProblem(t, w); problem.Detail != "Tag not found" {
			t.Fatalf("%s detail=%q", label, problem.Detail)
		}
		assertNoPrivateData(t, label, w)
	}
	if f.scans.revealed != 0 {
		t.Fatal("hidden contact must not mark the phone as revealed")
	}
}
