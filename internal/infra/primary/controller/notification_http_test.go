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
)

const testNotifID = "9c2f3a1e-5b7d-4c8e-9a10-2b3c4d5e6f70"

type stubNotificationUC struct {
	page    authBaseDomain.Page[domain.Notification]
	count   int
	updated int
	err     error
	calls   []string
	owner   string
	id      string
	req     authBaseDomain.PageRequest
}

func (s *stubNotificationUC) List(_ context.Context, ownerID string, page authBaseDomain.PageRequest) (authBaseDomain.Page[domain.Notification], error) {
	s.calls = append(s.calls, "List")
	s.owner, s.req = ownerID, page
	return s.page, s.err
}
func (s *stubNotificationUC) UnreadCount(_ context.Context, ownerID string) (int, error) {
	s.calls = append(s.calls, "UnreadCount")
	s.owner = ownerID
	return s.count, s.err
}
func (s *stubNotificationUC) MarkRead(_ context.Context, ownerID, id string) error {
	s.calls = append(s.calls, "MarkRead")
	s.owner, s.id = ownerID, id
	return s.err
}
func (s *stubNotificationUC) MarkAllRead(_ context.Context, ownerID string) (int, error) {
	s.calls = append(s.calls, "MarkAllRead")
	s.owner = ownerID
	return s.updated, s.err
}

func notificationEngine(uc *stubNotificationUC, authenticated bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	auth := guardAuthz{}
	if authenticated {
		auth.user = domain.User{UserGeneric: authBaseDomain.NewUserGeneric(publicTestOwnerID, "Ada", "ada@example.com", nil, true, false)}
	}
	engine := gin.New()
	NewNotificationController(auth, uc, engine).RunController()
	return engine
}

func sampleNotification(id string, read bool) domain.Notification {
	at := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	n := domain.Notification{
		ID: id, UserID: publicTestOwnerID, PetID: testPetID, TagID: publicTestTagID, ScanID: "scan-1", MessageID: "msg-secret",
		Kind: domain.NotificationLocationShared, CreatedAt: at,
		Payload: domain.NotificationPayload{
			PetName: "Luna", TagCode: publicTestCode, ScannedAt: at,
			Location: &domain.ScanLocation{Lat: -33.45, Lng: -70.66, AccuracyM: 20, At: at},
		},
	}
	if read {
		n.ReadAt = &at
	}
	return n
}

func TestNotificationsRequireAuth(t *testing.T) {
	uc := &stubNotificationUC{}
	engine := notificationEngine(uc, false)
	for _, route := range [][2]string{
		{http.MethodGet, "/v1/notifications"},
		{http.MethodGet, "/v1/notifications/unread-count"},
		{http.MethodPost, "/v1/notifications/" + testNotifID + "/read"},
		{http.MethodPost, "/v1/notifications/read-all"},
	} {
		if w := doPet(engine, route[0], route[1], "", ""); w.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status=%d", route[0], route[1], w.Code)
		}
	}
	if len(uc.calls) != 0 {
		t.Fatalf("calls=%v", uc.calls)
	}
}

func TestNotificationsOwnerScoped(t *testing.T) {
	uc := &stubNotificationUC{page: authBaseDomain.Page[domain.Notification]{
		Items:   []domain.Notification{sampleNotification(testNotifID, false), sampleNotification("1d2e3f40-5a6b-4c7d-8e9f-0a1b2c3d4e5f", true)},
		HasNext: true,
	}}
	w := doPet(notificationEngine(uc, true), http.MethodGet, "/v1/notifications?limit=2", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if uc.owner != publicTestOwnerID || uc.req.Limit != 2 {
		t.Fatalf("owner=%q req=%+v", uc.owner, uc.req)
	}
	body := w.Body.String()
	for _, secret := range []string{publicTestOwnerID, "msg-secret", "userId", "messageId"} {
		if strings.Contains(body, secret) {
			t.Fatalf("response leaks %q: %s", secret, body)
		}
	}
	var resp struct {
		Data struct {
			Limit      int    `json:"limit"`
			HasNext    bool   `json:"hasNext"`
			NextCursor string `json:"nextCursor"`
			Data       []struct {
				ID        string `json:"id"`
				Kind      string `json:"kind"`
				PetID     string `json:"petId"`
				TagID     string `json:"tagId"`
				ScanID    string `json:"scanId"`
				ReadAt    string `json:"readAt"`
				CreatedAt string `json:"createdAt"`
				Payload   struct {
					PetName  string `json:"petName"`
					Location struct {
						AccuracyM int `json:"accuracyM"`
					} `json:"location"`
				} `json:"payload"`
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	d := resp.Data
	if d.Limit != 2 || !d.HasNext || d.NextCursor == "" || len(d.Data) != 2 {
		t.Fatalf("page=%+v", d)
	}
	first, second := d.Data[0], d.Data[1]
	if first.ID != testNotifID || first.Kind != "location_shared" || first.PetID != testPetID || first.ScanID != "scan-1" ||
		first.ReadAt != "" || first.CreatedAt != "2026-10-25T09:00:00Z" || first.Payload.PetName != "Luna" || first.Payload.Location.AccuracyM != 20 {
		t.Fatalf("first=%+v", first)
	}
	if second.ReadAt != "2026-10-25T09:00:00Z" {
		t.Fatalf("second readAt=%q", second.ReadAt)
	}

	empty := doPet(notificationEngine(&stubNotificationUC{}, true), http.MethodGet, "/v1/notifications", "", "")
	if !strings.Contains(empty.Body.String(), `"data":[]`) || strings.Contains(empty.Body.String(), "nextCursor") {
		t.Fatalf("empty body=%s", empty.Body.String())
	}
	if bad := doPet(notificationEngine(&stubNotificationUC{}, true), http.MethodGet, "/v1/notifications?limit=0", "", ""); bad.Code != http.StatusBadRequest {
		t.Fatalf("bad limit status=%d", bad.Code)
	}
}

func TestUnreadCount(t *testing.T) {
	uc := &stubNotificationUC{count: 4}
	w := doPet(notificationEngine(uc, true), http.MethodGet, "/v1/notifications/unread-count", "", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"data":{"count":4}`) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if uc.owner != publicTestOwnerID || len(uc.calls) != 1 || uc.calls[0] != "UnreadCount" {
		t.Fatalf("owner=%q calls=%v", uc.owner, uc.calls)
	}
}

func TestMarkReadForeign404(t *testing.T) {
	uc := &stubNotificationUC{}
	w := doPet(notificationEngine(uc, true), http.MethodPost, "/v1/notifications/"+testNotifID+"/read", "", "")
	if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if uc.owner != publicTestOwnerID || uc.id != testNotifID {
		t.Fatalf("owner=%q id=%q", uc.owner, uc.id)
	}
	foreign := doPet(notificationEngine(&stubNotificationUC{err: domain.ErrNotFound}, true), http.MethodPost, "/v1/notifications/"+testNotifID+"/read", "", "")
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("foreign status=%d body=%s", foreign.Code, foreign.Body.String())
	}
}

func TestReadAll(t *testing.T) {
	uc := &stubNotificationUC{updated: 3}
	w := doPet(notificationEngine(uc, true), http.MethodPost, "/v1/notifications/read-all", "", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"data":{"updated":3}`) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if len(uc.calls) != 1 || uc.calls[0] != "MarkAllRead" || uc.owner != publicTestOwnerID {
		t.Fatalf("calls=%v owner=%q", uc.calls, uc.owner)
	}
}
