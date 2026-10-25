package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gin-gonic/gin"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/httpapi"
)

const (
	tagTestBaseURL = "https://hachikapp.example"
	tagTestBatchID = "5b6c7d8e-9f0a-4b1c-8d2e-3f4a5b6c7d8e"
)

type stubTagUC struct {
	err      error
	tag      domain.PetTag
	tags     []domain.PetTag
	batch    domain.TagBatch
	batches  authBaseDomain.Page[domain.TagBatch]
	calls    []string
	owner    string
	code     string
	petID    string
	actor    string
	label    string
	quantity int
	page     authBaseDomain.PageRequest
}

func (s *stubTagUC) CreateBatch(_ context.Context, actorID, label string, quantity int) (domain.TagBatch, error) {
	s.calls = append(s.calls, "CreateBatch")
	s.actor, s.label, s.quantity = actorID, label, quantity
	if s.err != nil {
		return domain.TagBatch{}, s.err
	}
	return s.batch, nil
}

func (s *stubTagUC) ListBatches(_ context.Context, page authBaseDomain.PageRequest) (authBaseDomain.Page[domain.TagBatch], error) {
	s.calls = append(s.calls, "ListBatches")
	s.page = page
	return s.batches, s.err
}

func (s *stubTagUC) BatchTags(_ context.Context, batchID string) ([]domain.PetTag, error) {
	s.calls = append(s.calls, "BatchTags")
	if s.err != nil {
		return nil, s.err
	}
	if batchID != tagTestBatchID {
		return nil, nil
	}
	return s.tags, nil
}

func (s *stubTagUC) ListPetTags(_ context.Context, ownerID, petID string) ([]domain.PetTag, error) {
	s.calls = append(s.calls, "ListPetTags")
	s.owner, s.petID = ownerID, petID
	return s.tags, s.err
}

func (s *stubTagUC) CreateDigital(_ context.Context, ownerID, petID string) (domain.PetTag, error) {
	s.calls = append(s.calls, "CreateDigital")
	s.owner, s.petID = ownerID, petID
	return s.tag, s.err
}

func (s *stubTagUC) Activate(_ context.Context, ownerID, code, petID string) (domain.PetTag, error) {
	s.calls = append(s.calls, "Activate")
	s.owner, s.code, s.petID = ownerID, code, petID
	return s.tag, s.err
}

func (s *stubTagUC) Disable(_ context.Context, ownerID, petID, tagID string) (domain.PetTag, error) {
	s.calls = append(s.calls, "Disable")
	s.owner, s.petID, s.code = ownerID, petID, tagID
	return s.tag, s.err
}

type stubScanHistoryUC struct {
	page  authBaseDomain.Page[domain.ScanWithMessages]
	err   error
	owner string
	petID string
	req   authBaseDomain.PageRequest
}

func (s *stubScanHistoryUC) View(context.Context, string) (domain.PublicTagView, error) {
	panic("unused")
}
func (s *stubScanHistoryUC) ShareLocation(context.Context, string, string, float64, float64, int) error {
	panic("unused")
}
func (s *stubScanHistoryUC) Photo(context.Context, string) (domain.PetPhoto, error) { panic("unused") }
func (s *stubScanHistoryUC) RecordScan(context.Context, string, string, string, string) (domain.ScanReceipt, error) {
	panic("unused")
}
func (s *stubScanHistoryUC) SendMessage(context.Context, string, string, domain.FinderMessageInput) error {
	panic("unused")
}
func (s *stubScanHistoryUC) RevealContact(context.Context, string, string) (domain.PublicContact, error) {
	panic("unused")
}
func (s *stubScanHistoryUC) ListPetScans(_ context.Context, ownerID, petID string, page authBaseDomain.PageRequest) (authBaseDomain.Page[domain.ScanWithMessages], error) {
	s.owner, s.petID, s.req = ownerID, petID, page
	return s.page, s.err
}

func tagEngine(uc *stubTagUC, scans *stubScanHistoryUC, authenticated bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	auth := guardAuthz{}
	if authenticated {
		auth.user = domain.User{UserGeneric: authBaseDomain.NewUserGeneric(publicTestOwnerID, "Ada", "ada@example.com", nil, true, false)}
	}
	if scans == nil {
		scans = &stubScanHistoryUC{}
	}
	engine := gin.New()
	engine.Use(httpapi.LimitRequestBody(httpapi.DefaultMaxBodyBytes))
	NewTagController(auth, uc, scans, tagTestBaseURL, engine).RunController()
	return engine
}

func sampleTag() domain.PetTag {
	activated := time.Date(2026, 10, 18, 21, 0, 0, 0, time.UTC)
	return domain.PetTag{
		ID: publicTestTagID, Code: publicTestCode, Kind: domain.TagKindDigital, Status: domain.TagStatusActive,
		BatchID: tagTestBatchID, PetID: testPetID, OwnerID: publicTestOwnerID,
		ActivatedAt: &activated, CreatedAt: activated,
	}
}

func TestTagRoutesRequireAuthentication(t *testing.T) {
	uc := &stubTagUC{}
	engine := tagEngine(uc, nil, false)
	for _, route := range [][2]string{
		{http.MethodGet, "/v1/pets/" + testPetID + "/tags"},
		{http.MethodPost, "/v1/pets/" + testPetID + "/tags"},
		{http.MethodPost, "/v1/pets/" + testPetID + "/tags/" + publicTestTagID + "/disable"},
		{http.MethodPost, "/v1/tags/" + publicTestCode + "/activate"},
		{http.MethodGet, "/v1/pets/" + testPetID + "/scans"},
	} {
		if w := doPet(engine, route[0], route[1], "", ""); w.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status=%d", route[0], route[1], w.Code)
		}
	}
	if len(uc.calls) != 0 {
		t.Fatalf("calls=%v", uc.calls)
	}
}

func TestActivateConflict409(t *testing.T) {
	cases := []struct {
		err    error
		status int
		detail string
	}{
		{domain.ErrTagNotAssignable, http.StatusConflict, "This tag is already linked"},
		{domain.ErrTagLimitReached, http.StatusConflict, "The pet already has 5 active tags"},
		{domain.ErrNotFound, http.StatusNotFound, ""},
	}
	for _, tc := range cases {
		uc := &stubTagUC{err: tc.err}
		w := doPet(tagEngine(uc, nil, true), http.MethodPost, "/v1/tags/k7m2-q9xr4p/activate", `{"petId":"`+testPetID+`"}`, "application/json")
		if w.Code != tc.status {
			t.Fatalf("%v status=%d body=%s", tc.err, w.Code, w.Body.String())
		}
		if tc.detail != "" && decodeProblem(t, w).Detail != tc.detail {
			t.Fatalf("%v detail=%q", tc.err, decodeProblem(t, w).Detail)
		}
		if uc.owner != publicTestOwnerID || uc.code != "k7m2-q9xr4p" || uc.petID != testPetID {
			t.Fatalf("activate args owner=%q code=%q pet=%q", uc.owner, uc.code, uc.petID)
		}
	}

	uc := &stubTagUC{tag: sampleTag()}
	w := doPet(tagEngine(uc, nil, true), http.MethodPost, "/v1/tags/"+publicTestCode+"/activate", `{"petId":"`+testPetID+`"}`, "application/json")
	if w.Code != http.StatusOK {
		t.Fatalf("activate status=%d body=%s", w.Code, w.Body.String())
	}
	if bad := doPet(tagEngine(&stubTagUC{}, nil, true), http.MethodPost, "/v1/tags/"+publicTestCode+"/activate", `{"petId":1}`, "application/json"); bad.Code != http.StatusBadRequest {
		t.Fatalf("bad body status=%d", bad.Code)
	}
}

func TestOwnerTagResponses(t *testing.T) {
	uc := &stubTagUC{tag: sampleTag(), tags: []domain.PetTag{sampleTag()}}
	engine := tagEngine(uc, nil, true)

	created := doPet(engine, http.MethodPost, "/v1/pets/"+testPetID+"/tags", "", "")
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"id": publicTestTagID, "code": publicTestCode, "kind": "digital", "status": "active", "petId": testPetID,
		"activatedAt": "2026-10-18T21:00:00Z", "createdAt": "2026-10-18T21:00:00Z", "url": tagTestBaseURL + "/t/" + publicTestCode,
	}
	for key, value := range want {
		if env.Data[key] != value {
			t.Fatalf("tag[%s]=%v want %v", key, env.Data[key], value)
		}
	}
	for _, key := range []string{"ownerId", "batchId", "disabledAt"} {
		if _, ok := env.Data[key]; ok {
			t.Fatalf("tag must not expose %s: %s", key, created.Body.String())
		}
	}

	list := doPet(engine, http.MethodGet, "/v1/pets/"+testPetID+"/tags", "", "")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"code":"`+publicTestCode+`"`) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	disabled := doPet(engine, http.MethodPost, "/v1/pets/"+testPetID+"/tags/"+publicTestTagID+"/disable", "", "")
	if disabled.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", disabled.Code, disabled.Body.String())
	}
	if uc.owner != publicTestOwnerID || uc.petID != testPetID || uc.code != publicTestTagID {
		t.Fatalf("disable args owner=%q pet=%q tag=%q", uc.owner, uc.petID, uc.code)
	}

	limit := doPet(tagEngine(&stubTagUC{err: domain.ErrTagLimitReached}, nil, true), http.MethodPost, "/v1/pets/"+testPetID+"/tags", "", "")
	if limit.Code != http.StatusConflict {
		t.Fatalf("limit status=%d", limit.Code)
	}
	missing := doPet(tagEngine(&stubTagUC{err: domain.ErrNotFound}, nil, true), http.MethodPost, "/v1/pets/"+testPetID+"/tags/"+publicTestTagID+"/disable", "", "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("disable missing status=%d", missing.Code)
	}
}

func TestPetScansOmitsHashes(t *testing.T) {
	scannedAt := time.Date(2026, 10, 18, 21, 0, 0, 0, time.UTC)
	revealed := scannedAt.Add(time.Minute)
	scans := &stubScanHistoryUC{page: authBaseDomain.Page[domain.ScanWithMessages]{
		HasNext: true,
		Items: []domain.ScanWithMessages{{
			TagScan: domain.TagScan{
				ID: "33333333-3333-3333-3333-000000000001", TagID: publicTestTagID, PetID: testPetID, ScannedAt: scannedAt,
				VisitorHash: "visitor-hash-secret", ScanTokenHash: "token-hash-secret", IPPrefix: "192.0.2.0/24",
				UserAgentFamily: "ios", PhoneRevealedAt: &revealed,
			},
			TagCode: publicTestCode,
			Messages: []domain.TagMessage{{
				ID: "44444444-4444-4444-4444-000000000001", Body: "La encontré", FinderName: "Camila", FinderPhone: "+56 9 1111 2222", CreatedAt: scannedAt,
			}},
		}},
	}}
	engine := tagEngine(&stubTagUC{}, scans, true)
	w := doPet(engine, http.MethodGet, "/v1/pets/"+testPetID+"/scans?limit=1", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, secret := range []string{"visitor-hash-secret", "token-hash-secret", "192.0.2", "visitorHash", "ipPrefix", publicTestTagID} {
		if strings.Contains(body, secret) {
			t.Fatalf("scans leak %q: %s", secret, body)
		}
	}
	for _, want := range []string{`"tagCode":"` + publicTestCode + `"`, `"userAgentFamily":"ios"`, `"phoneRevealedAt":"2026-10-18T21:01:00Z"`, `"finderPhone":"+56 9 1111 2222"`, `"hasNext":true`, `"nextCursor":"`, `"limit":1`} {
		if !strings.Contains(body, want) {
			t.Fatalf("scans missing %s: %s", want, body)
		}
	}
	if scans.owner != publicTestOwnerID || scans.petID != testPetID || scans.req.Limit != 1 {
		t.Fatalf("args owner=%q pet=%q page=%+v", scans.owner, scans.petID, scans.req)
	}

	// The next cursor round-trips to the last scan's (scanned_at, id).
	var env struct {
		Data struct {
			NextCursor string `json:"nextCursor"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	key, err := httpapi.DecodeCursor(env.Data.NextCursor)
	if err != nil || !key.CreatedAt.Equal(scannedAt) || key.ID != "33333333-3333-3333-3333-000000000001" {
		t.Fatalf("cursor=%+v err=%v", key, err)
	}

	if bad := doPet(engine, http.MethodGet, "/v1/pets/"+testPetID+"/scans?limit=0", "", ""); bad.Code != http.StatusBadRequest {
		t.Fatalf("bad limit status=%d", bad.Code)
	}
	missing := doPet(tagEngine(&stubTagUC{}, &stubScanHistoryUC{err: domain.ErrNotFound}, true), http.MethodGet, "/v1/pets/"+testPetID+"/scans", "", "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing status=%d", missing.Code)
	}
}

func TestTagRoutesCoexistWithPetRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := domain.User{UserGeneric: authBaseDomain.NewUserGeneric(publicTestOwnerID, "Ada", "ada@example.com", nil, true, false)}
	auth := guardAuthz{user: user}
	engine := gin.New()
	pet := publicTestPet(domain.PetVisibilityAlways)
	// Registering every controller on one engine must not panic on route conflicts.
	NewPetController(auth, &stubPetUC{pet: pet}, engine, nil).RunController()
	NewTagController(auth, &stubTagUC{tags: []domain.PetTag{sampleTag()}}, &stubScanHistoryUC{}, tagTestBaseURL, engine).RunController()
	NewTagAdminController(auth, &stubTagUC{}, tagTestBaseURL, engine).RunController()
	NewPublicTagController(&stubScanHistoryUC{}, nil, nil, []byte(publicTestPepper), engine).RunController()

	for _, path := range []string{"/v1/pets/" + testPetID, "/v1/pets/" + testPetID + "/tags", "/v1/pets/" + testPetID + "/scans"} {
		if w := doPet(engine, http.MethodGet, path, "", ""); w.Code != http.StatusOK {
			t.Fatalf("GET %s status=%d body=%s", path, w.Code, w.Body.String())
		}
	}
}

func TestOwnerScansIncludeLocation(t *testing.T) {
	scannedAt := time.Date(2026, 10, 18, 21, 0, 0, 0, time.UTC)
	at := scannedAt.Add(2 * time.Minute)
	scans := &stubScanHistoryUC{page: authBaseDomain.Page[domain.ScanWithMessages]{
		Items: []domain.ScanWithMessages{
			{TagScan: domain.TagScan{ID: "33333333-3333-3333-3333-000000000001", ScannedAt: scannedAt,
				Location: &domain.ScanLocation{Lat: -33.4372, Lng: -70.6506, AccuracyM: 35, At: at}}, TagCode: publicTestCode},
			{TagScan: domain.TagScan{ID: "33333333-3333-3333-3333-000000000002", ScannedAt: scannedAt}, TagCode: publicTestCode},
		},
	}}
	w := doPet(tagEngine(&stubTagUC{}, scans, true), http.MethodGet, "/v1/pets/"+testPetID+"/scans", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var env struct {
		Data struct {
			Data []map[string]any `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	items := env.Data.Data
	if len(items) == 0 {
		var flat struct {
			Data []map[string]any `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &flat)
		items = flat.Data
	}
	if len(items) != 2 {
		t.Fatalf("body=%s", w.Body.String())
	}
	loc, ok := items[0]["location"].(map[string]any)
	if !ok || loc["lat"] != -33.4372 || loc["lng"] != -70.6506 || loc["accuracyM"] != float64(35) || loc["at"] != "2026-10-18T21:02:00Z" {
		t.Fatalf("location=%v", items[0]["location"])
	}
	if _, present := items[1]["location"]; present {
		t.Fatalf("scan without location must omit the key: %v", items[1])
	}
}
