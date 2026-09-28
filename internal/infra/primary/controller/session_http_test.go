package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto"
	sessionout "github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto/outputs/sessions"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/ginadapter"
	"github.com/gin-gonic/gin"
)

type stubSessionUC struct {
	page           authBaseDomain.Page[domain.Session]
	listActor      string
	listFamily     string
	revokeActor    string
	revokeFamily   string
	revokeAllActor string
	revokedCurrent bool
}

func (s *stubSessionUC) ListMine(_ context.Context, actorID, currentFamilyID string, _ authBaseDomain.PageRequest) (authBaseDomain.Page[domain.Session], error) {
	s.listActor = actorID
	s.listFamily = currentFamilyID
	return s.page, nil
}

func (s *stubSessionUC) RevokeMine(_ context.Context, actorID, familyID, _ string) (bool, error) {
	s.revokeActor = actorID
	s.revokeFamily = familyID
	return s.revokedCurrent, nil
}

func (s *stubSessionUC) RevokeAllMine(_ context.Context, actorID string) error {
	s.revokeAllActor = actorID
	return nil
}

type stubAuthz struct {
	user domain.User
}

func (s stubAuthz) AuthorizeJWT() func(*ginadapter.Context) {
	return func(c *ginadapter.Context) { c.Next() }
}

func (s stubAuthz) PoliciesGuard(
	fn func(*ginadapter.Context),
	_ func(domain.User, string, authBaseDomain.OperationEnum) bool,
	_ string,
	_ authBaseDomain.OperationEnum,
) func(*ginadapter.Context) {
	return fn
}

func (s stubAuthz) GetUserToken(authBaseCore.Context) (domain.User, bool) {
	return s.user, s.user.GetId() != ""
}

func (s stubAuthz) IsAuthorized(domain.User, func(domain.User, string, authBaseDomain.OperationEnum) bool, string, authBaseDomain.OperationEnum) bool {
	return false
}

func TestSessionListUsesActorFromJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	uc := &stubSessionUC{page: authBaseDomain.Page[domain.Session]{Items: []domain.Session{{
		FamilyID: "family-1", Current: true, IssuedAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Hour), ClientLabel: "Mozilla/5.0",
	}}}}
	user := domain.User{UserGeneric: authBaseDomain.NewUserGeneric("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "Ada", "ada@example.com", nil, false, true)}
	ctrl := NewSessionController(stubAuthz{user: user}, uc, nil, gin.New(), nil)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.GET("/v1/users/me/sessions", ctrl.List())
	req := httptest.NewRequest(http.MethodGet, "/v1/users/me/sessions", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if uc.listActor != user.GetId() {
		t.Fatalf("actor=%s", uc.listActor)
	}
	var envelope dto.ResponseDto
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(envelope.Data)
	if strings.Contains(strings.ToLower(string(raw)), "token_hash") || strings.Contains(string(raw), "token_id") {
		t.Fatalf("leaked token material: %s", raw)
	}
	var page sessionout.SessionsPageResponse
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Data) != 1 || page.Data[0].FamilyID != "family-1" || !page.Data[0].Current {
		t.Fatalf("page=%+v", page)
	}
}

func TestSessionRevokeOneUsesPathFamilyAndActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uc := &stubSessionUC{revokedCurrent: false}
	user := domain.User{UserGeneric: authBaseDomain.NewUserGeneric("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "Ada", "ada@example.com", nil, false, true)}
	ctrl := NewSessionController(stubAuthz{user: user}, uc, nil, gin.New(), nil)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.DELETE("/v1/users/me/sessions/:familyId", ctrl.RevokeOne())
	req := httptest.NewRequest(http.MethodDelete, "/v1/users/me/sessions/family-other", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if uc.revokeActor != user.GetId() || uc.revokeFamily != "family-other" {
		t.Fatalf("actor=%s family=%s", uc.revokeActor, uc.revokeFamily)
	}
	if !strings.Contains(w.Body.String(), `"revokedCurrent":false`) {
		t.Fatalf("body=%s", w.Body.String())
	}
}

func TestSessionRevokeAllMarksCurrent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uc := &stubSessionUC{}
	user := domain.User{UserGeneric: authBaseDomain.NewUserGeneric("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "Ada", "ada@example.com", nil, false, true)}
	ctrl := NewSessionController(stubAuthz{user: user}, uc, nil, gin.New(), nil)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.DELETE("/v1/users/me/sessions", ctrl.RevokeAll())
	req := httptest.NewRequest(http.MethodDelete, "/v1/users/me/sessions", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if uc.revokeAllActor != user.GetId() {
		t.Fatalf("actor=%s", uc.revokeAllActor)
	}
	if !strings.Contains(w.Body.String(), `"revokedCurrent":true`) {
		t.Fatalf("body=%s", w.Body.String())
	}
}
