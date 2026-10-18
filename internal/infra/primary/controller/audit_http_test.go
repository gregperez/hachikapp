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
	"github.com/WilsonSayago/base-golang/internal/core/authz"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto"
	auditout "github.com/WilsonSayago/base-golang/internal/infra/primary/controller/dto/outputs/audits"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/ginadapter"
	"github.com/WilsonSayago/base-golang/internal/infra/primary/httpapi"
	"github.com/gin-gonic/gin"
)

type stubAuditUC struct {
	page   authBaseDomain.Page[domain.AuditEvent]
	filter domain.AuditListFilter
	err    error
}

func (s *stubAuditUC) List(_ context.Context, filter domain.AuditListFilter, _ authBaseDomain.PageRequest) (authBaseDomain.Page[domain.AuditEvent], error) {
	s.filter = filter
	if s.err != nil {
		return authBaseDomain.Page[domain.AuditEvent]{}, s.err
	}
	return s.page, nil
}

type guardAuthz struct {
	user domain.User
	deny bool
}

func (s guardAuthz) AuthorizeJWT() func(*ginadapter.Context) {
	return func(c *ginadapter.Context) {
		if s.user.GetId() == "" {
			httpapi.WriteProblem(c.Context, httpapi.Unauthorized("Authentication required", httpapi.RequestID(c.Context)))
			return
		}
		c.Next()
	}
}

func (s guardAuthz) PoliciesGuard(
	fn func(*ginadapter.Context),
	_ func(domain.User, string, authBaseDomain.OperationEnum) bool,
	_ string,
	_ authBaseDomain.OperationEnum,
) func(*ginadapter.Context) {
	if s.deny {
		return func(c *ginadapter.Context) {
			httpapi.WriteProblem(c.Context, httpapi.Forbidden("You do not have permission to perform this action", httpapi.RequestID(c.Context)))
		}
	}
	return fn
}

func (s guardAuthz) GetUserToken(authBaseCore.Context) (domain.User, bool) {
	return s.user, s.user.GetId() != ""
}

func (s guardAuthz) IsAuthorized(domain.User, func(domain.User, string, authBaseDomain.OperationEnum) bool, string, authBaseDomain.OperationEnum) bool {
	return !s.deny
}

func TestAuditListRequiresAuthAndOmitsSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	uc := &stubAuditUC{page: authBaseDomain.Page[domain.AuditEvent]{Items: []domain.AuditEvent{{
		ID: "11111111-1111-1111-1111-111111111111", OccurredAt: now, Category: domain.AuditCategoryPrivilege,
		Action: "assign_roles", Outcome: "success", ActorID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		SubjectType: domain.AuditSubjectUser, SubjectID: "bbbbbbbb-bbbb-cccc-dddd-eeeeeeeeeeee",
		Metadata: map[string]any{"roleIds": []string{"cccccccc-cccc-cccc-dddd-eeeeeeeeeeee"}},
	}}}}
	user := domain.User{UserGeneric: authBaseDomain.NewUserGeneric("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "Ada", "ada@example.com", nil, true, true)}
	ctrl := NewAuditController(guardAuthz{user: user}, uc, gin.New(), nil)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.GET("/v1/audit-events", ginadapter.Middleware(ctrl.authorization.PoliciesGuard(ginadapter.Handler(ctrl.List()), nil, authz.EntityAuditEvents, authz.Read)))
	req := httptest.NewRequest(http.MethodGet, "/v1/audit-events?action=assign_roles", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if uc.filter.Action != "assign_roles" {
		t.Fatalf("filter=%+v", uc.filter)
	}
	body := w.Body.String()
	for _, leaked := range []string{"password", "ada@example.com", "Bearer ", "postgres://"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("leaked %q in %s", leaked, body)
		}
	}
	var envelope dto.ResponseDto
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(envelope.Data)
	var page auditout.AuditEventsPageResponse
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Data) != 1 || page.Data[0].Action != "assign_roles" {
		t.Fatalf("page=%+v", page)
	}
}

func TestAuditListForbiddenWithoutPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := domain.User{UserGeneric: authBaseDomain.NewUserGeneric("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "Ada", "ada@example.com", nil, false, true)}
	ctrl := NewAuditController(guardAuthz{user: user, deny: true}, &stubAuditUC{}, gin.New(), nil)
	w := httptest.NewRecorder()
	engine := gin.New()
	engine.GET("/v1/audit-events", ginadapter.Middleware(ctrl.authorization.AuthorizeJWT()), ginadapter.Middleware(ctrl.authorization.PoliciesGuard(ginadapter.Handler(ctrl.List()), nil, authz.EntityAuditEvents, authz.Read)))
	req := httptest.NewRequest(http.MethodGet, "/v1/audit-events", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestAuditListUnauthorizedWithoutUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := NewAuditController(guardAuthz{}, &stubAuditUC{}, gin.New(), nil)
	w := httptest.NewRecorder()
	engine := gin.New()
	engine.GET("/v1/audit-events", ginadapter.Middleware(ctrl.authorization.AuthorizeJWT()), ginadapter.Middleware(ctrl.authorization.PoliciesGuard(ginadapter.Handler(ctrl.List()), nil, authz.EntityAuditEvents, authz.Read)))
	req := httptest.NewRequest(http.MethodGet, "/v1/audit-events", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestAuditListRejectsInvalidCursorAndTime(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := domain.User{UserGeneric: authBaseDomain.NewUserGeneric("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "Ada", "ada@example.com", nil, true, true)}
	ctrl := NewAuditController(guardAuthz{user: user}, &stubAuditUC{}, gin.New(), nil)
	engine := gin.New()
	engine.GET("/v1/audit-events", ctrl.List())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/audit-events?cursor=not-a-cursor", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("cursor status=%d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/audit-events?from=yesterday", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("from status=%d body=%s", w.Code, w.Body.String())
	}
}
