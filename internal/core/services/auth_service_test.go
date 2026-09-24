package services_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/WilsonSayago/authBase/v3/core"
	"github.com/WilsonSayago/authBase/v3/core/domain"
	"github.com/WilsonSayago/authBase/v3/core/port"
	"github.com/WilsonSayago/authBase/v3/core/services"
	"github.com/WilsonSayago/authBase/v3/infra/config/properties"
	"github.com/WilsonSayago/authBase/v3/infra/secundary"
)

type memIdentity struct {
	mu    sync.Mutex
	users map[string]domain.UserGeneric
	creds map[string]port.CredentialRecord
}

func newMemIdentity() *memIdentity {
	return &memIdentity{users: map[string]domain.UserGeneric{}, creds: map[string]port.CredentialRecord{}}
}

func (m *memIdentity) add(u domain.UserGeneric, user, hash string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.GetId()] = u
	m.creds[user] = port.CredentialRecord{UserID: u.GetId(), Username: user, PasswordHash: hash, Active: u.GetActive()}
}

func (m *memIdentity) FindByID(ctx context.Context, id string) (domain.UserGeneric, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return domain.UserGeneric{}, core.ErrNotFound
	}
	return u, nil
}

func (m *memIdentity) FindCredentialsByUsername(ctx context.Context, username string) (port.CredentialRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.creds[username]
	if !ok {
		return port.CredentialRecord{}, core.ErrNotFound
	}
	return c, nil
}

func (m *memIdentity) setActive(id string, active bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.users[id]
	m.users[id] = domain.NewUserGeneric(u.GetId(), u.GetName(), u.GetEmail(), nil, u.GetIsAdmin(), active)
	for k, c := range m.creds {
		if c.UserID == id {
			c.Active = active
			m.creds[k] = c
		}
	}
}

type memRefresh struct {
	mu       sync.Mutex
	byHash   map[[32]byte]domain.RefreshSession
	consumed map[[32]byte]struct{}
	revoked  map[string]struct{}
}

func newMemRefresh() *memRefresh {
	return &memRefresh{
		byHash:   map[[32]byte]domain.RefreshSession{},
		consumed: map[[32]byte]struct{}{},
		revoked:  map[string]struct{}{},
	}
}

func (s *memRefresh) Create(ctx context.Context, session domain.RefreshSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byHash[session.TokenHash] = session
	return nil
}

func (s *memRefresh) Rotate(ctx context.Context, currentHash [32]byte, next domain.RefreshSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.byHash[currentHash]
	if !ok {
		return core.ErrRefreshNotFound
	}
	if _, r := s.revoked[cur.FamilyID]; r {
		return core.ErrRefreshFamilyRevoked
	}
	if _, c := s.consumed[currentHash]; c {
		return core.ErrRefreshConsumed
	}
	s.consumed[currentHash] = struct{}{}
	s.byHash[next.TokenHash] = next
	return nil
}

func (s *memRefresh) RevokeFamily(ctx context.Context, familyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revoked[familyID] = struct{}{}
	return nil
}

func testJWT() properties.Jwt {
	return properties.Jwt{
		SecretKey:        strings.Repeat("a", 32),
		RefreshSecret:    strings.Repeat("b", 32),
		ExpirationTime:   1,
		RefreshTokenTime: 24,
		Issuer:           "test-issuer",
		Audience:         "test-audience",
		LeewaySeconds:    0,
	}
}

func TestAuthenticationLoginRefreshReplayInactive(t *testing.T) {
	t.Parallel()
	id := newMemIdentity()
	store := newMemRefresh()
	validate := secundary.NewValidationService()
	hash, err := validate.HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	user := domain.NewUserGeneric("u1", "n", "u@example.com", nil, false, true)
	id.add(user, "u@example.com", hash)

	svc, err := services.NewAuthenticationService[domain.UserGeneric](id, id, store, validate, testJWT())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	_, _, err = svc.Login(ctx, "u@example.com", "wrong")
	if !errors.Is(err, core.ErrInvalidCredentials) {
		t.Fatalf("bad login: %v", err)
	}

	access, refresh, err := svc.Login(ctx, "u@example.com", "correct-horse")
	if err != nil || access == "" || refresh == "" {
		t.Fatalf("login: %v", err)
	}

	access2, refresh2, err := svc.RefreshToken(ctx, refresh)
	if err != nil || access2 == "" || refresh2 == "" || refresh2 == refresh {
		t.Fatalf("refresh: %v a=%q r=%q", err, access2, refresh2)
	}

	_, _, err = svc.RefreshToken(ctx, refresh)
	if !errors.Is(err, core.ErrInvalidRefresh) {
		t.Fatalf("replay: %v", err)
	}

	_, _, err = svc.RefreshToken(ctx, refresh2)
	if !errors.Is(err, core.ErrInvalidRefresh) {
		t.Fatalf("descendant after replay revoke: %v", err)
	}

	_, refresh3, err := svc.Login(ctx, "u@example.com", "correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	accessActive, _, err := svc.Login(ctx, "u@example.com", "correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	id.setActive("u1", false)
	_, _, err = svc.RefreshToken(ctx, refresh3)
	if !errors.Is(err, core.ErrInvalidRefresh) {
		t.Fatalf("inactive refresh: %v", err)
	}
	_, err = svc.ValidateToken(ctx, accessActive)
	if !errors.Is(err, core.ErrInactiveIdentity) {
		t.Fatalf("inactive validate: %v", err)
	}
}

func TestRevokeFamilyIdempotent(t *testing.T) {
	t.Parallel()
	id := newMemIdentity()
	store := newMemRefresh()
	validate := secundary.NewValidationService()
	hash, _ := validate.HashPassword("pw")
	id.add(domain.NewUserGeneric("u1", "n", "u@example.com", nil, false, true), "u@example.com", hash)
	svc, err := services.NewAuthenticationService[domain.UserGeneric](id, id, store, validate, testJWT())
	if err != nil {
		t.Fatal(err)
	}
	_, refresh, err := svc.Login(context.Background(), "u@example.com", "pw")
	if err != nil {
		t.Fatal(err)
	}
	tm, _ := services.NewTokenManager(testJWT())
	claims, err := tm.ParseRefresh(refresh)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeRefreshFamily(context.Background(), claims.FamilyID); err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeRefreshFamily(context.Background(), claims.FamilyID); err != nil {
		t.Fatal(err)
	}
}
