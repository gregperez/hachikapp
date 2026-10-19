package controller

import (
	"encoding/csv"
	"net/http"
	"strings"
	"testing"
	"time"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gin-gonic/gin"
	"github.com/gregperez/hachikapp-backend/internal/core/authz"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/ginadapter"
	"github.com/gregperez/hachikapp-backend/internal/infra/primary/httpapi"
)

// permissionAuthz grants only the listed (entity, operation) pairs, like a role
// with explicit permissions behind PoliciesGuard.
type permissionAuthz struct {
	user    domain.User
	granted map[string]bool
}

func (p permissionAuthz) AuthorizeJWT() func(*ginadapter.Context) {
	return guardAuthz{user: p.user}.AuthorizeJWT()
}

func (p permissionAuthz) PoliciesGuard(
	fn func(*ginadapter.Context),
	_ func(domain.User, string, authBaseDomain.OperationEnum) bool,
	entity string,
	op authBaseDomain.OperationEnum,
) func(*ginadapter.Context) {
	return func(c *ginadapter.Context) {
		if !p.granted[entity+":"+string(op)] {
			httpapi.WriteProblem(c.Context, httpapi.Forbidden("You do not have permission to perform this action", httpapi.RequestID(c.Context)))
			return
		}
		fn(c)
	}
}

func (p permissionAuthz) GetUserToken(authBaseCore.Context) (domain.User, bool) {
	return p.user, p.user.GetId() != ""
}

func (p permissionAuthz) IsAuthorized(domain.User, func(domain.User, string, authBaseDomain.OperationEnum) bool, string, authBaseDomain.OperationEnum) bool {
	return false
}

func adminTagEngine(uc *stubTagUC, granted ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	auth := permissionAuthz{
		user:    domain.User{UserGeneric: authBaseDomain.NewUserGeneric(publicTestOwnerID, "Ada", "ada@example.com", nil, true, false)},
		granted: map[string]bool{},
	}
	for _, g := range granted {
		auth.granted[g] = true
	}
	engine := gin.New()
	engine.Use(httpapi.LimitRequestBody(httpapi.DefaultMaxBodyBytes))
	NewTagAdminController(auth, uc, tagTestBaseURL, engine).RunController()
	return engine
}

var (
	tagsCreate = authz.EntityTags + ":" + string(authz.Create)
	tagsRead   = authz.EntityTags + ":" + string(authz.Read)
)

func TestAdminBatchRequiresTagsPermission(t *testing.T) {
	routes := []struct {
		method, path, body, needs string
	}{
		{http.MethodGet, "/v1/admin/tag-batches", "", tagsRead},
		{http.MethodPost, "/v1/admin/tag-batches", `{"label":"Lote","quantity":2}`, tagsCreate},
		{http.MethodGet, "/v1/admin/tag-batches/" + tagTestBatchID + "/codes.csv", "", tagsRead},
	}
	for _, route := range routes {
		// No permission, or only another entity's permission → 403.
		for _, granted := range [][]string{nil, {authz.EntityUsers + ":" + string(authz.Create), authz.EntityUsers + ":" + string(authz.Read)}} {
			uc := &stubTagUC{}
			w := doPet(adminTagEngine(uc, granted...), route.method, route.path, route.body, "application/json")
			if w.Code != http.StatusForbidden {
				t.Fatalf("%s %s granted=%v status=%d", route.method, route.path, granted, w.Code)
			}
			if len(uc.calls) != 0 {
				t.Fatalf("forbidden request reached use case: %v", uc.calls)
			}
		}
		// The matching TAGS permission is enough.
		uc := &stubTagUC{tags: []domain.PetTag{sampleTag()}, batch: domain.TagBatch{ID: tagTestBatchID, Label: "Lote", Quantity: 2}}
		w := doPet(adminTagEngine(uc, route.needs), route.method, route.path, route.body, "application/json")
		if w.Code >= 400 {
			t.Fatalf("%s %s with %s status=%d body=%s", route.method, route.path, route.needs, w.Code, w.Body.String())
		}
	}
	// Read does not allow creating.
	if w := doPet(adminTagEngine(&stubTagUC{}, tagsRead), http.MethodPost, "/v1/admin/tag-batches", `{"label":"Lote","quantity":2}`, "application/json"); w.Code != http.StatusForbidden {
		t.Fatalf("read-only create status=%d", w.Code)
	}
}

func TestBatchCSV(t *testing.T) {
	tags := []domain.PetTag{{Code: "K7M2Q9XR4P"}, {Code: "0123456789"}}
	uc := &stubTagUC{tags: tags}
	engine := adminTagEngine(uc, tagsRead)

	w := doPet(engine, http.MethodGet, "/v1/admin/tag-batches/"+tagTestBatchID+"/codes.csv", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "text/csv; charset=utf-8" {
		t.Fatalf("content-type=%q", got)
	}
	if got := w.Header().Get("Content-Disposition"); got != `attachment; filename="placas-`+tagTestBatchID+`.csv"` {
		t.Fatalf("content-disposition=%q", got)
	}
	records, err := csv.NewReader(strings.NewReader(w.Body.String())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"code", "url"},
		{"K7M2Q9XR4P", tagTestBaseURL + "/t/K7M2Q9XR4P"},
		{"0123456789", tagTestBaseURL + "/t/0123456789"},
	}
	if len(records) != len(want) {
		t.Fatalf("records=%v", records)
	}
	for i := range want {
		if records[i][0] != want[i][0] || records[i][1] != want[i][1] {
			t.Fatalf("row %d=%v want %v", i, records[i], want[i])
		}
	}

	for _, id := range []string{"99999999-9999-9999-9999-999999999999", "not-a-uuid"} {
		missing := doPet(engine, http.MethodGet, "/v1/admin/tag-batches/"+id+"/codes.csv", "", "")
		if missing.Code != http.StatusNotFound {
			t.Fatalf("unknown batch %s status=%d", id, missing.Code)
		}
	}
}

func TestAdminBatchCreateAndList(t *testing.T) {
	created := time.Date(2026, 10, 18, 21, 0, 0, 0, time.UTC)
	batch := domain.TagBatch{ID: tagTestBatchID, Label: "Lote octubre", Quantity: 100, CreatedBy: publicTestOwnerID, CreatedAt: created}
	uc := &stubTagUC{batch: batch, batches: authBaseDomain.Page[domain.TagBatch]{Items: []domain.TagBatch{batch}, HasNext: true}}
	engine := adminTagEngine(uc, tagsCreate, tagsRead)

	w := doPet(engine, http.MethodPost, "/v1/admin/tag-batches", `{"label":"Lote octubre","quantity":100}`, "application/json")
	if w.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}
	if uc.actor != publicTestOwnerID || uc.label != "Lote octubre" || uc.quantity != 100 {
		t.Fatalf("create args actor=%q label=%q qty=%d", uc.actor, uc.label, uc.quantity)
	}
	for _, want := range []string{`"id":"` + tagTestBatchID + `"`, `"label":"Lote octubre"`, `"quantity":100`, `"createdAt":"2026-10-18T21:00:00Z"`} {
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("create body missing %s: %s", want, w.Body.String())
		}
	}

	invalid := doPet(adminTagEngine(&stubTagUC{err: domain.ValidateTagBatch("", 1001)}, tagsCreate), http.MethodPost, "/v1/admin/tag-batches", `{"label":"","quantity":1001}`, "application/json")
	if invalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid status=%d body=%s", invalid.Code, invalid.Body.String())
	}
	if problem := decodeProblem(t, invalid); len(problem.Errors) != 2 {
		t.Fatalf("issues=%+v", problem.Errors)
	}

	list := doPet(engine, http.MethodGet, "/v1/admin/tag-batches?limit=5", "", "")
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	if uc.page.Limit != 5 || !strings.Contains(list.Body.String(), `"hasNext":true`) || !strings.Contains(list.Body.String(), `"nextCursor":"`) {
		t.Fatalf("list page=%+v body=%s", uc.page, list.Body.String())
	}
}
