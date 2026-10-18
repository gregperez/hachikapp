package services_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	authBasePorts "github.com/WilsonSayago/authBase/v4/core/port"
	"github.com/WilsonSayago/authBase/v4/infra/secundary"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/core/services"
)

type memUsers struct {
	mu           sync.Mutex
	byID         map[string]domain.User
	admins       int
	roles        map[string]string
	saved        []domain.User
	revoked      []string
	passwordHash string
}

func newMemUsers() *memUsers {
	return &memUsers{
		byID:  map[string]domain.User{},
		roles: map[string]string{"admin": "role-admin"},
	}
}

func (m *memUsers) put(u domain.User) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID[u.GetId()] = u
	if u.GetIsAdmin() && u.GetActive() {
		m.admins++
	}
}

func (m *memUsers) FindByID(ctx context.Context, id string) (domain.User, error) {
	return m.FindById(ctx, id)
}
func (m *memUsers) FindCredentialsByUsername(ctx context.Context, username string) (authBasePorts.CredentialRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.byID {
		if u.GetEmail() == username {
			return authBasePorts.CredentialRecord{
				UserID: u.GetId(), Username: username, PasswordHash: "hash", Active: u.GetActive(),
			}, nil
		}
	}
	return authBasePorts.CredentialRecord{}, errors.New("not found")
}
func (m *memUsers) Save(ctx context.Context, u domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u.GetId() == "" {
		u = domain.WithPlainPassword(domain.User{UserGeneric: authBaseDomain.NewUserGeneric(
			"new-id", u.GetName(), u.GetEmail(), u.GetRole(), u.GetIsAdmin(), true,
		)}, u.PlainPassword())
	}
	m.byID[u.GetId()] = u
	m.saved = append(m.saved, u)
	if u.GetIsAdmin() {
		m.admins++
	}
	return nil
}
func (m *memUsers) FindAll(
	ctx context.Context,
	request authBaseDomain.PageRequest,
) (authBaseDomain.Page[domain.User], error) {
	return authBaseDomain.Page[domain.User]{}, nil
}
func (m *memUsers) FindById(ctx context.Context, id string) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.byID[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}
func (m *memUsers) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	return domain.User{}, nil
}
func (m *memUsers) FindFullById(ctx context.Context, id string) (domain.User, error) {
	return m.FindById(ctx, id)
}
func (m *memUsers) Update(ctx context.Context, u domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID[u.GetId()] = u
	return nil
}
func (m *memUsers) SetActive(ctx context.Context, id string, active bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	m.byID[id] = domain.User{UserGeneric: authBaseDomain.NewUserGeneric(
		u.GetId(), u.GetName(), u.GetEmail(), u.GetRole(), u.GetIsAdmin(), active,
	)}
	return nil
}
func (m *memUsers) CountAdmins(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, u := range m.byID {
		if u.GetIsAdmin() && u.GetActive() {
			n++
		}
	}
	return n, nil
}
func (m *memUsers) SaveFirstAdmin(ctx context.Context, u domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, existing := range m.byID {
		if existing.GetIsAdmin() && existing.GetActive() {
			n++
		}
	}
	if n > 0 {
		return domain.ErrAdminExists
	}
	if u.GetId() == "" {
		u = domain.WithPlainPassword(domain.User{UserGeneric: authBaseDomain.NewUserGeneric(
			"new-id", u.GetName(), u.GetEmail(), u.GetRole(), true, true,
		)}, u.PlainPassword())
	}
	m.byID[u.GetId()] = u
	m.saved = append(m.saved, u)
	return nil
}
func (m *memUsers) DemoteAdminIfNotLast(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	if !u.GetIsAdmin() {
		return nil
	}
	n := 0
	for _, other := range m.byID {
		if other.GetId() != id && other.GetIsAdmin() && other.GetActive() {
			n++
		}
	}
	if n < 1 {
		return domain.ErrLastAdmin
	}
	m.byID[id] = domain.User{UserGeneric: authBaseDomain.NewUserGeneric(
		u.GetId(), u.GetName(), u.GetEmail(), u.GetRole(), false, u.GetActive(),
	)}
	return nil
}
func (m *memUsers) DeactivateAdminIfNotLast(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	if !u.GetActive() {
		return nil
	}
	if u.GetIsAdmin() {
		n := 0
		for _, other := range m.byID {
			if other.GetId() != id && other.GetIsAdmin() && other.GetActive() {
				n++
			}
		}
		if n < 1 {
			return domain.ErrLastAdmin
		}
	}
	m.byID[id] = domain.User{UserGeneric: authBaseDomain.NewUserGeneric(
		u.GetId(), u.GetName(), u.GetEmail(), u.GetRole(), u.GetIsAdmin(), false,
	)}
	return nil
}
func (m *memUsers) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.passwordHash = passwordHash
	return nil
}
func (m *memUsers) UpdatePasswordAndRevokeSessions(ctx context.Context, id, passwordHash string) error {
	if err := m.UpdatePassword(ctx, id, passwordHash); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revoked = append(m.revoked, id)
	return nil
}
func (m *memUsers) DeactivateAndRevokeSessions(ctx context.Context, id string) error {
	if err := m.SetActive(ctx, id, false); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revoked = append(m.revoked, id)
	return nil
}
func (m *memUsers) DeactivateAdminAndRevokeSessions(ctx context.Context, id string) error {
	if err := m.DeactivateAdminIfNotLast(ctx, id); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revoked = append(m.revoked, id)
	return nil
}
func (m *memUsers) FindRoleIDByName(ctx context.Context, name string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.roles[name]
	if !ok {
		return "", domain.ErrNotFound
	}
	return id, nil
}

type memAudit struct {
	mu   sync.Mutex
	logs []string
}

type passwordValidation struct{}

func (passwordValidation) HashPassword(password string) (string, error) {
	return "new:" + password, nil
}

func (passwordValidation) CheckPassword(hashedPassword, password string) bool {
	return hashedPassword == "hash" && password == "CurrentHorse1"
}

func (a *memAudit) PrivilegeChange(_ context.Context, action, actorID, subjectID string, _ []string, _ *bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.logs = append(a.logs, action+":"+actorID+":"+subjectID)
}

func TestCreateStripsPrivileges(t *testing.T) {
	t.Parallel()
	repo := newMemUsers()
	svc := services.NewUserService(repo, secundary.NewValidationService(), ports.NoopAuditor{})
	err := svc.Create(context.Background(), domain.WithPlainPassword(domain.User{
		UserGeneric: authBaseDomain.NewUserGeneric("", "Ada", "ada@ex.com",
			[]authBaseDomain.Role{{Base: authBaseDomain.Base{Id: "r1"}}}, true, true),
	}, "CorrectHorseBattery1"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("saved=%d", len(repo.saved))
	}
	got := repo.saved[0]
	if got.GetIsAdmin() {
		t.Fatal("Create must not persist isAdmin")
	}
	if len(got.GetRole()) != 0 {
		t.Fatal("Create must not persist roles")
	}
}

func TestUpdateOwnProfileForbiddenForOtherUser(t *testing.T) {
	t.Parallel()
	repo := newMemUsers()
	repo.put(domain.User{UserGeneric: authBaseDomain.NewUserGeneric("u1", "Ada", "ada@ex.com", nil, false, true)})
	svc := services.NewUserService(repo, nil, nil)
	if err := svc.UpdateOwnProfile(context.Background(), "u2", "u1", "New"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("err=%v", err)
	}
}

func TestUpdateOwnProfileKeepsPrivileges(t *testing.T) {
	t.Parallel()
	repo := newMemUsers()
	roles := []authBaseDomain.Role{{Base: authBaseDomain.Base{Id: "r1", Active: true}}}
	repo.put(domain.User{UserGeneric: authBaseDomain.NewUserGeneric("u1", "Ada", "ada@ex.com", roles, true, true)})
	svc := services.NewUserService(repo, nil, nil)
	if err := svc.UpdateOwnProfile(context.Background(), "u1", "u1", "Ada Lovelace"); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.FindById(context.Background(), "u1")
	if got.GetName() != "Ada Lovelace" {
		t.Fatalf("name=%s", got.GetName())
	}
	if !got.GetIsAdmin() || len(got.GetRole()) != 1 {
		t.Fatal("profile update must preserve privileges")
	}
}

func TestLastAdminProtection(t *testing.T) {
	t.Parallel()
	repo := newMemUsers()
	repo.put(domain.User{UserGeneric: authBaseDomain.NewUserGeneric("a1", "Admin", "a@ex.com", nil, true, true)})
	repo.admins = 1
	audit := &memAudit{}
	svc := services.NewUserService(repo, nil, audit)
	ctx := context.Background()
	if err := svc.SetAdminFlag(ctx, "a1", "a1", false); !errors.Is(err, domain.ErrLastAdmin) {
		t.Fatalf("SetAdminFlag err=%v", err)
	}
	if err := svc.SetActive(ctx, "a1", "a1", false); !errors.Is(err, domain.ErrLastAdmin) {
		t.Fatalf("SetActive err=%v", err)
	}
}

func TestAssignRolesAudits(t *testing.T) {
	t.Parallel()
	repo := newMemUsers()
	repo.put(domain.User{UserGeneric: authBaseDomain.NewUserGeneric("u1", "Ada", "ada@ex.com", nil, false, true)})
	audit := &memAudit{}
	svc := services.NewUserService(repo, nil, audit)
	if err := svc.AssignRoles(context.Background(), "admin-1", "u1", []string{"role-x"}); err != nil {
		t.Fatal(err)
	}
	if len(audit.logs) != 1 || audit.logs[0] != "assign_roles:admin-1:u1" {
		t.Fatalf("audit=%v", audit.logs)
	}
}

func TestAdminBootstrapOnce(t *testing.T) {
	t.Parallel()
	repo := newMemUsers()
	boot := services.NewAdminBootstrap(repo)
	ctx := context.Background()
	if err := boot.Run(ctx, "Admin", "admin@ex.com", "CorrectHorseBattery1"); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := boot.Run(ctx, "Admin", "admin2@ex.com", "CorrectHorseBattery1"); !errors.Is(err, domain.ErrAdminExists) {
		t.Fatalf("second: %v", err)
	}
}

func TestAdminBootstrapRejectsWeakPassword(t *testing.T) {
	t.Parallel()
	boot := services.NewAdminBootstrap(newMemUsers())
	if err := boot.Run(context.Background(), "Admin", "admin@ex.com", "weak"); !errors.Is(err, domain.ErrWeakPassword) {
		t.Fatalf("err=%v", err)
	}
}

func TestSetActiveIdempotent(t *testing.T) {
	t.Parallel()
	repo := newMemUsers()
	repo.put(domain.User{UserGeneric: authBaseDomain.NewUserGeneric("u1", "Ada", "ada@ex.com", nil, false, false)})
	svc := services.NewUserService(repo, nil, &memAudit{})
	ctx := context.Background()
	if err := svc.SetActive(ctx, "a", "u1", true); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetActive(ctx, "a", "u1", true); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.FindById(ctx, "u1")
	if !got.GetActive() {
		t.Fatal("expected active")
	}
	if len(repo.revoked) != 0 {
		t.Fatalf("activation revoked sessions: %v", repo.revoked)
	}
}

func TestChangePasswordRevokesSessions(t *testing.T) {
	t.Parallel()
	repo := newMemUsers()
	repo.put(domain.User{UserGeneric: authBaseDomain.NewUserGeneric(
		"u1", "Ada", "ada@ex.com", nil, false, true,
	)})
	svc := services.NewUserService(repo, passwordValidation{}, nil)

	if err := svc.ChangePassword(
		context.Background(),
		"u1",
		"CurrentHorse1",
		"ReplacementHorse2",
	); err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if repo.passwordHash != "new:ReplacementHorse2" {
		t.Fatalf("password hash = %q", repo.passwordHash)
	}
	if len(repo.revoked) != 1 || repo.revoked[0] != "u1" {
		t.Fatalf("revoked = %v, want [u1]", repo.revoked)
	}
}

func TestDeactivateRevokesSessionsButActivationDoesNot(t *testing.T) {
	t.Parallel()
	repo := newMemUsers()
	repo.put(domain.User{UserGeneric: authBaseDomain.NewUserGeneric(
		"u1", "Ada", "ada@ex.com", nil, false, true,
	)})
	svc := services.NewUserService(repo, nil, nil)
	ctx := context.Background()

	if err := svc.SetActive(ctx, "admin", "u1", false); err != nil {
		t.Fatalf("deactivate error = %v", err)
	}
	if len(repo.revoked) != 1 || repo.revoked[0] != "u1" {
		t.Fatalf("revoked after deactivate = %v", repo.revoked)
	}
	if err := svc.SetActive(ctx, "admin", "u1", true); err != nil {
		t.Fatalf("activate error = %v", err)
	}
	if len(repo.revoked) != 1 {
		t.Fatalf("activation added revocation: %v", repo.revoked)
	}
}

func TestDeactivateAdminRevokesSessions(t *testing.T) {
	t.Parallel()
	repo := newMemUsers()
	repo.put(domain.User{UserGeneric: authBaseDomain.NewUserGeneric(
		"a1", "Admin One", "a1@ex.com", nil, true, true,
	)})
	repo.put(domain.User{UserGeneric: authBaseDomain.NewUserGeneric(
		"a2", "Admin Two", "a2@ex.com", nil, true, true,
	)})
	svc := services.NewUserService(repo, nil, nil)

	if err := svc.SetActive(context.Background(), "a2", "a1", false); err != nil {
		t.Fatalf("SetActive(admin false) error = %v", err)
	}
	if len(repo.revoked) != 1 || repo.revoked[0] != "a1" {
		t.Fatalf("revoked = %v, want [a1]", repo.revoked)
	}
}

func TestAdminFlagChangesDoNotRevokeSessions(t *testing.T) {
	t.Parallel()
	repo := newMemUsers()
	repo.put(domain.User{UserGeneric: authBaseDomain.NewUserGeneric(
		"a1", "Admin One", "a1@ex.com", nil, true, true,
	)})
	repo.put(domain.User{UserGeneric: authBaseDomain.NewUserGeneric(
		"a2", "Admin Two", "a2@ex.com", nil, true, true,
	)})
	repo.put(domain.User{UserGeneric: authBaseDomain.NewUserGeneric(
		"u1", "Ada", "ada@ex.com", nil, false, true,
	)})
	svc := services.NewUserService(repo, nil, nil)
	ctx := context.Background()

	if err := svc.SetAdminFlag(ctx, "a2", "a1", false); err != nil {
		t.Fatalf("demote error = %v", err)
	}
	if err := svc.SetAdminFlag(ctx, "a2", "u1", true); err != nil {
		t.Fatalf("promote error = %v", err)
	}
	if len(repo.revoked) != 0 {
		t.Fatalf("admin flag changes revoked sessions: %v", repo.revoked)
	}
}
