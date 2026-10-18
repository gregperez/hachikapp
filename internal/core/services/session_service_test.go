package services_test

import (
	"context"
	"testing"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/services"
)

type memSessions struct {
	listed   []domain.Session
	revoked  map[string][]string
	users    []string
	listUser string
}

func (m *memSessions) ListActiveByUser(_ context.Context, userID string, _ authBaseDomain.PageRequest) (authBaseDomain.Page[domain.Session], error) {
	m.listUser = userID
	items := append([]domain.Session{}, m.listed...)
	return authBaseDomain.Page[domain.Session]{Items: items}, nil
}

func (m *memSessions) RevokeFamilyForUser(_ context.Context, userID, familyID string) error {
	if m.revoked == nil {
		m.revoked = map[string][]string{}
	}
	m.revoked[userID] = append(m.revoked[userID], familyID)
	return nil
}

func (m *memSessions) RevokeUser(_ context.Context, userID string) error {
	m.users = append(m.users, userID)
	return nil
}

func TestSessionServiceMarksCurrentAndScopesActor(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	port := &memSessions{listed: []domain.Session{
		{FamilyID: "family-a", IssuedAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Hour)},
		{FamilyID: "family-b", IssuedAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Hour)},
	}}
	svc := services.NewSessionService(port)
	page, err := svc.ListMine(context.Background(), "user-1", "family-b", authBaseDomain.PageRequest{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if port.listUser != "user-1" {
		t.Fatalf("listed user=%s", port.listUser)
	}
	if len(page.Items) != 2 || !page.Items[1].Current || page.Items[0].Current {
		t.Fatalf("current flags: %+v", page.Items)
	}
}

func TestSessionServiceRevokeMineReportsCurrentFamily(t *testing.T) {
	t.Parallel()
	port := &memSessions{}
	svc := services.NewSessionService(port)
	current, err := svc.RevokeMine(context.Background(), "user-1", "family-a", "family-a")
	if err != nil || !current {
		t.Fatalf("current=%v err=%v", current, err)
	}
	other, err := svc.RevokeMine(context.Background(), "user-1", "family-b", "family-a")
	if err != nil || other {
		t.Fatalf("other=%v err=%v", other, err)
	}
	if got := port.revoked["user-1"]; len(got) != 2 {
		t.Fatalf("revoked=%v", got)
	}
}

func TestSessionServiceRevokeAllUsesActorOnly(t *testing.T) {
	t.Parallel()
	port := &memSessions{}
	svc := services.NewSessionService(port)
	if err := svc.RevokeAllMine(context.Background(), "user-1"); err != nil {
		t.Fatal(err)
	}
	if len(port.users) != 1 || port.users[0] != "user-1" {
		t.Fatalf("users=%v", port.users)
	}
}
