package services_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	authBaseCore "github.com/WilsonSayago/authBase/v4/core"
	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	authBasePorts "github.com/WilsonSayago/authBase/v4/core/port"
	"github.com/WilsonSayago/authBase/v4/infra/secundary"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/core/services"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/oidccrypto"
)

type memOIDC struct {
	mu          sync.Mutex
	txns        map[string]domain.OIDCTransaction
	grants      map[string]string
	identities  map[string]domain.ExternalIdentity
	users       map[string]domain.User
	creds       map[string]authBasePorts.CredentialRecord
	principal   domain.ExternalPrincipal
	exchangeErr error
}

func (m *memOIDC) key(issuer, subject string) string { return issuer + "\x00" + subject }

func (m *memOIDC) Create(_ context.Context, tx domain.OIDCTransaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.txns == nil {
		m.txns = map[string]domain.OIDCTransaction{}
	}
	tx.ID = string(tx.StateHash)
	m.txns[string(tx.StateHash)] = tx
	return nil
}

func (m *memOIDC) ConsumeByStateHash(_ context.Context, stateHash []byte, at time.Time) (domain.OIDCTransaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tx, ok := m.txns[string(stateHash)]
	if !ok {
		return domain.OIDCTransaction{}, domain.ErrOIDCInvalidState
	}
	if tx.ConsumedAt != nil {
		return domain.OIDCTransaction{}, domain.ErrOIDCReplay
	}
	if !tx.ExpiresAt.After(at) {
		return domain.OIDCTransaction{}, domain.ErrOIDCExpired
	}
	now := at
	tx.ConsumedAt = &now
	m.txns[string(stateHash)] = tx
	return tx, nil
}

func (m *memOIDC) AttachGrant(_ context.Context, id string, grantHash []byte, userID string, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tx, ok := m.txns[id]
	if !ok {
		return domain.ErrOIDCInvalidState
	}
	tx.GrantHash = grantHash
	tx.GrantExpiresAt = &expiresAt
	tx.ResolvedUserID = userID
	m.txns[id] = tx
	if m.grants == nil {
		m.grants = map[string]string{}
	}
	m.grants[string(grantHash)] = id
	return nil
}

func (m *memOIDC) ConsumeGrant(_ context.Context, grantHash []byte, at time.Time) (domain.OIDCTransaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.grants[string(grantHash)]
	if !ok {
		return domain.OIDCTransaction{}, domain.ErrOIDCInvalidGrant
	}
	tx := m.txns[id]
	if tx.GrantConsumedAt != nil {
		return domain.OIDCTransaction{}, domain.ErrOIDCReplay
	}
	if tx.GrantExpiresAt == nil || !tx.GrantExpiresAt.After(at) {
		return domain.OIDCTransaction{}, domain.ErrOIDCExpired
	}
	now := at
	tx.GrantConsumedAt = &now
	m.txns[id] = tx
	return tx, nil
}

func (m *memOIDC) PurgeExpired(context.Context, time.Time) (int64, error) { return 0, nil }

func (m *memOIDC) FindByIssuerSubject(_ context.Context, issuer, subject string) (domain.ExternalIdentity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	identity, ok := m.identities[m.key(issuer, subject)]
	if !ok {
		return domain.ExternalIdentity{}, domain.ErrNotFound
	}
	return identity, nil
}

func (m *memOIDC) Link(_ context.Context, identity domain.ExternalIdentity) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.identities == nil {
		m.identities = map[string]domain.ExternalIdentity{}
	}
	key := m.key(identity.Issuer, identity.Subject)
	if existing, ok := m.identities[key]; ok && existing.UserID != identity.UserID {
		return domain.ErrConflict
	}
	identity.ID = key
	m.identities[key] = identity
	return nil
}

func (m *memOIDC) MarkLogin(_ context.Context, id string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, identity := range m.identities {
		if identity.ID == id {
			identity.LastLoginAt = &at
			m.identities[key] = identity
			return nil
		}
	}
	return domain.ErrNotFound
}

func (m *memOIDC) Unlink(_ context.Context, userID, provider string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, identity := range m.identities {
		if identity.UserID == userID && identity.Provider == provider {
			delete(m.identities, key)
			return nil
		}
	}
	return domain.ErrNotFound
}

func (m *memOIDC) FindByID(_ context.Context, id string) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return user, nil
}

func (m *memOIDC) FindCredentialsByUsername(_ context.Context, username string) (authBasePorts.CredentialRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cred, ok := m.creds[username]
	if !ok {
		return authBasePorts.CredentialRecord{}, authBaseCore.ErrNotFound
	}
	return cred, nil
}

func (m *memOIDC) Name() string   { return "google" }
func (m *memOIDC) Issuer() string { return "https://accounts.example.test" }
func (m *memOIDC) AuthorizationURL(state, nonce, challenge string) (string, error) {
	return "https://accounts.example.test/auth?state=" + state + "&nonce=" + nonce + "&c=" + challenge, nil
}
func (m *memOIDC) Exchange(context.Context, string, string, string) (domain.ExternalPrincipal, error) {
	if m.exchangeErr != nil {
		return domain.ExternalPrincipal{}, m.exchangeErr
	}
	return m.principal, nil
}

type stubSessions struct {
	access, refresh string
}

func (s stubSessions) EstablishSession(context.Context, string) (string, string, error) {
	return s.access, s.refresh, nil
}

func testBox(t *testing.T) ports.OIDCSecrets {
	t.Helper()
	box, err := oidccrypto.NewBox(bytes32(0x3c))
	if err != nil {
		t.Fatal(err)
	}
	return box
}

func bytes32(b byte) []byte {
	out := make([]byte, 32)
	for i := range out {
		out[i] = b
	}
	return out
}

func newService(t *testing.T, mem *memOIDC) *services.OIDCService {
	t.Helper()
	svc, err := services.NewOIDCService(
		services.OIDCServiceConfig{Enabled: true, TransactionTTL: time.Minute, GrantTTL: time.Minute},
		[]ports.OIDCProvider{mem},
		map[string][]string{"google": {"https://app.example.com/auth/oidc"}},
		mem, mem, mem, testBox(t), stubSessions{access: "access-local", refresh: "refresh-local"},
		secundary.NewValidationService(),
		ports.NoopOIDCAuditor{},
	)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func seedUser(mem *memOIDC, id, email, password string, active bool) {
	validate := secundary.NewValidationService()
	hash, err := validate.HashPassword(password)
	if err != nil {
		panic(err)
	}
	mem.users = map[string]domain.User{
		id: {UserGeneric: authBaseDomain.NewUserGeneric(id, "Ada", email, nil, false, active)},
	}
	mem.creds = map[string]authBasePorts.CredentialRecord{
		email: {UserID: id, Username: email, PasswordHash: hash, Active: active},
	}
}

func TestOIDCLoginHappyPathAndReplay(t *testing.T) {
	t.Parallel()
	mem := &memOIDC{
		principal: domain.ExternalPrincipal{
			Provider: "google", Issuer: "https://accounts.example.test", Subject: "sub-1",
			Email: "ada@example.com", EmailVerified: true,
		},
	}
	seedUser(mem, "user-1", "ada@example.com", "CorrectHorseBattery1", true)
	mem.identities = map[string]domain.ExternalIdentity{
		"https://accounts.example.test\x00sub-1": {
			ID: "https://accounts.example.test\x00sub-1", Provider: "google",
			Issuer: "https://accounts.example.test", Subject: "sub-1", UserID: "user-1",
		},
	}
	svc := newService(t, mem)

	start, err := svc.StartLogin(context.Background(), "google", "https://app.example.com/auth/oidc")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(start.AuthorizationURL, "code_challenge") && !strings.Contains(start.AuthorizationURL, "&c=") {
		t.Fatalf("auth url=%s", start.AuthorizationURL)
	}
	cb, err := svc.FinishCallback(context.Background(), "google", start.State, start.State, "code-1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cb.RedirectURL, "eyJ") || strings.Contains(cb.RedirectURL, "access-local") {
		t.Fatalf("tokens leaked in redirect: %s", cb.RedirectURL)
	}
	grant := grantFrom(cb.RedirectURL)
	access, refresh, err := svc.ExchangeGrant(context.Background(), grant)
	if err != nil {
		t.Fatal(err)
	}
	if access != "access-local" || refresh != "refresh-local" {
		t.Fatalf("tokens=%s %s", access, refresh)
	}
	if _, err := svc.FinishCallback(context.Background(), "google", start.State, start.State, "code-1"); err != domain.ErrOIDCReplay {
		t.Fatalf("replay err=%v", err)
	}
	if _, _, err := svc.ExchangeGrant(context.Background(), grant); err != domain.ErrOIDCReplay && err != domain.ErrOIDCInvalidGrant {
		t.Fatalf("grant replay err=%v", err)
	}
}

func TestOIDCRejectsUnlinkedEmailMatch(t *testing.T) {
	t.Parallel()
	mem := &memOIDC{
		principal: domain.ExternalPrincipal{
			Provider: "google", Issuer: "https://accounts.example.test", Subject: "other-sub",
			Email: "ada@example.com", EmailVerified: true,
		},
	}
	seedUser(mem, "user-1", "ada@example.com", "CorrectHorseBattery1", true)
	svc := newService(t, mem)
	start, err := svc.StartLogin(context.Background(), "google", "https://app.example.com/auth/oidc")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.FinishCallback(context.Background(), "google", start.State, start.State, "code-1")
	if err != domain.ErrOIDCIdentityNotLinked {
		t.Fatalf("err=%v", err)
	}
}

func TestOIDCLinkConflictAndSuccess(t *testing.T) {
	t.Parallel()
	mem := &memOIDC{
		principal: domain.ExternalPrincipal{
			Provider: "google", Issuer: "https://accounts.example.test", Subject: "sub-2",
			Email: "ada@example.com", EmailVerified: true,
		},
	}
	seedUser(mem, "user-1", "ada@example.com", "CorrectHorseBattery1", true)
	mem.identities = map[string]domain.ExternalIdentity{
		"https://accounts.example.test\x00sub-2": {
			ID: "linked", Provider: "google", Issuer: "https://accounts.example.test",
			Subject: "sub-2", UserID: "other-user",
		},
	}
	svc := newService(t, mem)
	start, err := svc.StartLink(context.Background(), "google", "user-1", "CorrectHorseBattery1", "https://app.example.com/auth/oidc")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.FinishCallback(context.Background(), "google", start.State, start.State, "code-1")
	if err != domain.ErrOIDCIdentityConflict {
		t.Fatalf("conflict err=%v", err)
	}

	mem2 := &memOIDC{
		principal: domain.ExternalPrincipal{
			Provider: "google", Issuer: "https://accounts.example.test", Subject: "sub-3",
			Email: "ada@example.com", EmailVerified: true,
		},
	}
	seedUser(mem2, "user-1", "ada@example.com", "CorrectHorseBattery1", true)
	svc2 := newService(t, mem2)
	start, err = svc2.StartLink(context.Background(), "google", "user-1", "CorrectHorseBattery1", "https://app.example.com/auth/oidc")
	if err != nil {
		t.Fatal(err)
	}
	cb, err := svc2.FinishCallback(context.Background(), "google", start.State, start.State, "code-1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cb.RedirectURL, "oidc=linked") {
		t.Fatalf("redirect=%s", cb.RedirectURL)
	}
}

func TestOIDCRejectsUnknownDestinationAndDisabled(t *testing.T) {
	t.Parallel()
	mem := &memOIDC{}
	seedUser(mem, "user-1", "ada@example.com", "CorrectHorseBattery1", true)
	svc := newService(t, mem)
	if _, err := svc.StartLogin(context.Background(), "google", "https://evil.example/cb"); err != domain.ErrOIDCRedirectNotAllowed {
		t.Fatalf("err=%v", err)
	}
	disabled, err := services.NewOIDCService(services.OIDCServiceConfig{}, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := disabled.StartLogin(context.Background(), "google", ""); err != domain.ErrOIDCDisabled {
		t.Fatalf("disabled err=%v", err)
	}
}

func TestOIDCRejectsInactiveUser(t *testing.T) {
	t.Parallel()
	mem := &memOIDC{
		principal: domain.ExternalPrincipal{
			Provider: "google", Issuer: "https://accounts.example.test", Subject: "sub-1",
			Email: "ada@example.com", EmailVerified: true,
		},
	}
	seedUser(mem, "user-1", "ada@example.com", "CorrectHorseBattery1", false)
	mem.identities = map[string]domain.ExternalIdentity{
		"https://accounts.example.test\x00sub-1": {
			ID: "https://accounts.example.test\x00sub-1", Provider: "google",
			Issuer: "https://accounts.example.test", Subject: "sub-1", UserID: "user-1",
		},
	}
	svc := newService(t, mem)
	start, err := svc.StartLogin(context.Background(), "google", "https://app.example.com/auth/oidc")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.FinishCallback(context.Background(), "google", start.State, start.State, "code-1"); err != domain.ErrOIDCInactiveUser {
		t.Fatalf("err=%v", err)
	}
}

func TestOIDCRejectsExpiredGrant(t *testing.T) {
	t.Parallel()
	mem := &memOIDC{
		principal: domain.ExternalPrincipal{
			Provider: "google", Issuer: "https://accounts.example.test", Subject: "sub-1",
			Email: "ada@example.com", EmailVerified: true,
		},
	}
	seedUser(mem, "user-1", "ada@example.com", "CorrectHorseBattery1", true)
	mem.identities = map[string]domain.ExternalIdentity{
		"https://accounts.example.test\x00sub-1": {
			ID: "https://accounts.example.test\x00sub-1", Provider: "google",
			Issuer: "https://accounts.example.test", Subject: "sub-1", UserID: "user-1",
		},
	}
	svc := newService(t, mem)
	start, err := svc.StartLogin(context.Background(), "google", "https://app.example.com/auth/oidc")
	if err != nil {
		t.Fatal(err)
	}
	cb, err := svc.FinishCallback(context.Background(), "google", start.State, start.State, "code-1")
	if err != nil {
		t.Fatal(err)
	}
	mem.mu.Lock()
	past := time.Now().Add(-time.Second)
	for id, tx := range mem.txns {
		tx.GrantExpiresAt = &past
		mem.txns[id] = tx
	}
	mem.mu.Unlock()
	if _, _, err := svc.ExchangeGrant(context.Background(), grantFrom(cb.RedirectURL)); err != domain.ErrOIDCExpired {
		t.Fatalf("err=%v", err)
	}
}

func grantFrom(raw string) string {
	const prefix = "grant="
	i := strings.Index(raw, prefix)
	if i < 0 {
		return ""
	}
	return raw[i+len(prefix):]
}

var (
	_ ports.OIDCTransactionStore  = (*memOIDC)(nil)
	_ ports.ExternalIdentityStore = (*memOIDC)(nil)
	_ ports.OIDCProvider          = (*memOIDC)(nil)
	_ authBaseCore.SessionIssuer  = stubSessions{}
)
