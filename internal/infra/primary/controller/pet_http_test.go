package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gin-gonic/gin"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto"
	petout "github.com/gregperez/hachikapp-backend/internal/infra/primary/controller/dto/outputs/pets"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/httpapi"
)

const testPetID = "2f1c5a0e-6a43-4f8e-9a51-0c2f3f9b7d11"

type stubPetUC struct {
	owner   string
	details domain.PetDetails
	version int64
	photo   []byte
	err     error
	pet     domain.Pet
}

func (s *stubPetUC) result(ownerID string, details domain.PetDetails) (domain.Pet, error) {
	s.owner = ownerID
	s.details = details
	if s.err != nil {
		return domain.Pet{}, s.err
	}
	pet := s.pet
	if pet.ID == "" {
		pet = domain.Pet{ID: testPetID, OwnerID: ownerID, PetDetails: domain.NormalizePetDetails(details), Version: 1}
	}
	return pet, nil
}

func (s *stubPetUC) List(_ context.Context, ownerID string) ([]domain.Pet, error) {
	s.owner = ownerID
	if s.err != nil {
		return nil, s.err
	}
	return []domain.Pet{s.pet}, nil
}

func (s *stubPetUC) Get(_ context.Context, ownerID, _ string) (domain.Pet, error) {
	return s.result(ownerID, s.pet.PetDetails)
}

func (s *stubPetUC) Create(_ context.Context, ownerID string, details domain.PetDetails) (domain.Pet, error) {
	return s.result(ownerID, details)
}

func (s *stubPetUC) Update(_ context.Context, ownerID, _ string, version int64, details domain.PetDetails) (domain.Pet, error) {
	s.version = version
	return s.result(ownerID, details)
}

func (s *stubPetUC) Delete(_ context.Context, ownerID, _ string) error {
	s.owner = ownerID
	return s.err
}

func (s *stubPetUC) GetPhoto(_ context.Context, ownerID, _ string) (domain.PetPhoto, error) {
	s.owner = ownerID
	if s.err != nil {
		return domain.PetPhoto{}, s.err
	}
	return domain.PetPhoto{ContentType: "image/png", Data: []byte("png-bytes")}, nil
}

func (s *stubPetUC) PutPhoto(_ context.Context, ownerID, _ string, data []byte) (domain.Pet, error) {
	s.photo = data
	pet, err := s.result(ownerID, s.pet.PetDetails)
	pet.HasPhoto = true
	pet.PhotoUpdatedAt = time.UnixMilli(1760820000000).UTC()
	return pet, err
}

func (s *stubPetUC) DeletePhoto(_ context.Context, ownerID, _ string) (domain.Pet, error) {
	return s.result(ownerID, s.pet.PetDetails)
}

func petEngine(uc *stubPetUC, authenticated bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	auth := guardAuthz{}
	if authenticated {
		auth.user = domain.User{UserGeneric: authBaseDomain.NewUserGeneric("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "Ada", "ada@example.com", nil, true, false)}
	}
	engine := gin.New()
	engine.Use(httpapi.LimitRequestBody(httpapi.DefaultMaxBodyBytes))
	NewPetController(auth, uc, engine, nil).RunController()
	return engine
}

func doPet(engine *gin.Engine, method, path, body string, contentType string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	engine.ServeHTTP(w, req)
	return w
}

func decodeProblem(t *testing.T, w *httptest.ResponseRecorder) httpapi.Problem {
	t.Helper()
	var problem httpapi.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatalf("problem body: %v (%s)", err, w.Body.String())
	}
	return problem
}

const validPetJSON = `{
  "name": "Luna",
  "species": "dog",
  "birthDate": "2021-03-14",
  "chipNumber": "900123456789012",
  "health": {"allergies": ["Atún"]},
  "owner": {"fullName": "Wilson Sayago", "address": "Av. Siempre Viva 742", "comuna": "Ñuñoa", "phone": "+56 9 1234 5678"},
  "secondOwner": {"fullName": "Andrea", "phone": ""},
  "publicVisibility": {"breed": "always"}
}`

func TestPetRoutesRequireAuthentication(t *testing.T) {
	engine := petEngine(&stubPetUC{}, false)
	for _, route := range [][2]string{{http.MethodGet, "/v1/pets"}, {http.MethodPost, "/v1/pets"}, {http.MethodGet, "/v1/pets/" + testPetID + "/photo"}} {
		w := doPet(engine, route[0], route[1], "", "")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status=%d", route[0], route[1], w.Code)
		}
	}
}

func TestPetCreateMapsRequestAndScopesToCaller(t *testing.T) {
	uc := &stubPetUC{}
	w := doPet(petEngine(uc, true), http.MethodPost, "/v1/pets", validPetJSON, "application/json")
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if uc.owner != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Fatalf("owner=%q", uc.owner)
	}
	if uc.details.BirthDate == nil || domain.FormatPetBirthDate(uc.details.BirthDate) != "2021-03-14" {
		t.Fatalf("birthDate=%v", uc.details.BirthDate)
	}
	if uc.details.SecondOwner.FullName != "Andrea" || uc.details.PublicVisibility[domain.PetPublicBreed] != domain.PetVisibilityAlways {
		t.Fatalf("details=%+v", uc.details)
	}
	if got := w.Header().Get("Location"); got != "/v1/pets/"+testPetID {
		t.Fatalf("location=%q", got)
	}
	var envelope dto.ResponseDto
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(envelope.Data)
	var pet petout.PetResponse
	if err := json.Unmarshal(raw, &pet); err != nil {
		t.Fatal(err)
	}
	if pet.Name != "Luna" || pet.BirthDate != "2021-03-14" || pet.Health.Surgeries == nil || pet.PublicVisibility["photo"] != "always" {
		t.Fatalf("response=%+v", pet)
	}
	if pet.PhotoURL != "" {
		t.Fatalf("no photo expected, got %q", pet.PhotoURL)
	}
}

func TestPetCreateRejectsUnknownFieldsAndBadDates(t *testing.T) {
	engine := petEngine(&stubPetUC{}, true)
	w := doPet(engine, http.MethodPost, "/v1/pets", `{"name":"Luna","ownerId":"someone-else"}`, "application/json")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status=%d body=%s", w.Code, w.Body.String())
	}
	w = doPet(engine, http.MethodPost, "/v1/pets", `{"name":"Luna","birthDate":"14/03/2021"}`, "application/json")
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad date status=%d", w.Code)
	}
	if problem := decodeProblem(t, w); len(problem.Errors) != 1 || problem.Errors[0].Field != "birthDate" {
		t.Fatalf("fields=%+v", problem.Errors)
	}
}

func TestPetErrorsMapToStableProblems(t *testing.T) {
	cases := []struct {
		err    error
		status int
		field  string
	}{
		{&domain.PetValidationError{Issues: []domain.PetFieldIssue{{Field: "owner.phone", Code: "phone"}}}, http.StatusUnprocessableEntity, "owner.phone"},
		{domain.ErrDuplicateChip, http.StatusConflict, ""},
		{domain.ErrPetLimitReached, http.StatusConflict, ""},
		{domain.ErrStaleVersion, http.StatusConflict, ""},
		{domain.ErrNotFound, http.StatusNotFound, ""},
	}
	for _, tc := range cases {
		w := doPet(petEngine(&stubPetUC{err: tc.err}, true), http.MethodPut, "/v1/pets/"+testPetID, `{"version":2,"name":"Luna"}`, "application/json")
		if w.Code != tc.status {
			t.Fatalf("%v: status=%d body=%s", tc.err, w.Code, w.Body.String())
		}
		problem := decodeProblem(t, w)
		if tc.field != "" && (len(problem.Errors) != 1 || problem.Errors[0].Field != tc.field) {
			t.Fatalf("%v: fields=%+v", tc.err, problem.Errors)
		}
	}
	w := doPet(petEngine(&stubPetUC{err: domain.ErrStaleVersion}, true), http.MethodPut, "/v1/pets/"+testPetID, `{"version":2,"name":"Luna"}`, "application/json")
	if !strings.Contains(w.Body.String(), "The pet was updated by someone else") {
		t.Fatalf("stale detail: %s", w.Body.String())
	}
}

func TestPetUpdatePassesVersion(t *testing.T) {
	uc := &stubPetUC{}
	w := doPet(petEngine(uc, true), http.MethodPut, "/v1/pets/"+testPetID, `{"version":7,"name":"Luna","species":"cat","owner":{"fullName":"W","phone":"+56912345678"}}`, "application/json")
	if w.Code != http.StatusOK || uc.version != 7 || uc.details.Species != domain.PetSpeciesCat {
		t.Fatalf("status=%d version=%d details=%+v", w.Code, uc.version, uc.details)
	}
}

func TestPetDeleteReturnsNoContent(t *testing.T) {
	w := doPet(petEngine(&stubPetUC{}, true), http.MethodDelete, "/v1/pets/"+testPetID, "", "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestPetPhotoUploadAndDownload(t *testing.T) {
	uc := &stubPetUC{}
	engine := petEngine(uc, true)
	w := doPet(engine, http.MethodPut, "/v1/pets/"+testPetID+"/photo", "\x89PNG\r\n\x1a\nrest", "image/png")
	if w.Code != http.StatusOK {
		t.Fatalf("upload status=%d body=%s", w.Code, w.Body.String())
	}
	if !bytes.HasPrefix(uc.photo, []byte("\x89PNG")) {
		t.Fatalf("photo bytes not forwarded: %q", uc.photo)
	}
	if !strings.Contains(w.Body.String(), `"photoUrl":"/v1/pets/`+testPetID+`/photo?v=1760820000000"`) {
		t.Fatalf("photoUrl missing: %s", w.Body.String())
	}

	w = doPet(engine, http.MethodGet, "/v1/pets/"+testPetID+"/photo", "", "")
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/png" || w.Body.String() != "png-bytes" {
		t.Fatalf("download status=%d type=%q body=%q", w.Code, w.Header().Get("Content-Type"), w.Body.String())
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("photo must be served with nosniff")
	}
}

func TestPetPhotoUploadTooLarge(t *testing.T) {
	big := strings.Repeat("a", domain.MaxPetPhotoBytes+1)
	w := doPet(petEngine(&stubPetUC{}, true), http.MethodPut, "/v1/pets/"+testPetID+"/photo", big, "image/jpeg")
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestPetPhotoInvalidImage(t *testing.T) {
	w := doPet(petEngine(&stubPetUC{err: domain.ErrInvalidPetPhoto}, true), http.MethodPut, "/v1/pets/"+testPetID+"/photo", "<svg/>", "image/svg+xml")
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d", w.Code)
	}
	if problem := decodeProblem(t, w); len(problem.Errors) != 1 || problem.Errors[0].Field != "photo" {
		t.Fatalf("fields=%+v", problem.Errors)
	}
}
