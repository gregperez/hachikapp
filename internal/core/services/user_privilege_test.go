package services_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	authBaseDomain "github.com/WilsonSayago/authBase/v3/core/domain"
	authBasePorts "github.com/WilsonSayago/authBase/v3/core/port"
	"github.com/WilsonSayago/authBase/v3/infra/secundary"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/ports"
	"github.com/WilsonSayago/base-golang/internal/core/services"
)

type memUsers struct {
	mu     sync.Mutex
	byID   map[string]domain.User
	admins int
	roles  map[string]string // name -> id
	saved  []domain.User
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
	return m.FindById(id)
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
func (m *memUsers) Save(u domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u.GetId() == "" {
		u = domain.User{UserGeneric: authBaseDomain.NewUserGeneric(
			"new-id", u.GetName(), u.GetEmail(), u.GetRole(), u.GetIsAdmin(), true,
		)}
		u = domain.WithPlainPassword(u, u.PlainPassword())
	}
	m.byID[u.GetId()] = u
	m.saved = append(m.saved, u)
	if u.GetIsAdmin() {
		m.admins++
	}
	return nil
}
func (m *memUsers) FindAll(pageSize, offset int) ([]domain.User, int, error) {
	return nil, 0, nil
}
func (m *memUsers) FindById(id string) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.byID[id]
	if !ok {
		return domain.User{}, errors.New("errors.users.not-found")
	}
	return u, nil
}
func (m *memUsers) FindByEmail(email string) (domain.User, error) {
	return domain.User{}, nil
}
func (m *memUsers) FindFullById(id string) (domain.User, error) {
	return m.FindById(id)
}
func (m *memUsers) Update(u domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID[u.GetId()] = u
	return nil
}
func (m *memUsers) ChangeStatus(id string) error { return nil }
func (m *memUsers) CountAdmins() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.admins, nil
}
func (m *memUsers) UpdatePassword(id, passwordHash string) error { return nil }
func (m *memUsers) FindRoleIDByName(name string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.roles[name]
	if !ok {
		return "", errors.New("errors.roles.not-found")
	}
	return id, nil
}

type memAudit struct {
	mu   sync.Mutex
	logs []string
}

func (a *memAudit) PrivilegeChange(action, actorID, subjectID string, roleIDs []string, isAdmin *bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.logs = append(a.logs, action+":"+actorID+":"+subjectID)
}

func TestCreateStripsPrivileges(t *testing.T) {
	t.Parallel()
	repo := newMemUsers()
	svc := services.NewUserService(repo, secundary.NewValidationService(), ports.NoopAuditor{})
	err := svc.Create(domain.WithPlainPassword(domain.User{
		UserGeneric: authBaseDomain.NewUserGeneric("", "Ada", "ada@ex.com",
			[]authBaseDomain.Role{{Base: authBaseDomain.Base{Id: "r1"}}}, true, true),
	}, "CorrectHorse1"))
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
	if err := svc.UpdateOwnProfile("u2", "u1", "New"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("err=%v", err)
	}
}

func TestUpdateOwnProfileKeepsPrivileges(t *testing.T) {
	t.Parallel()
	repo := newMemUsers()
	roles := []authBaseDomain.Role{{Base: authBaseDomain.Base{Id: "r1", Active: true}}}
	repo.put(domain.User{UserGeneric: authBaseDomain.NewUserGeneric("u1", "Ada", "ada@ex.com", roles, true, true)})
	svc := services.NewUserService(repo, nil, nil)
	if err := svc.UpdateOwnProfile("u1", "u1", "Ada Lovelace"); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.FindById("u1")
	if got.GetName() != "Ada Lovelace" {
		t.Fatalf("name=%s", got.GetName())
	}
	if !got.GetIsAdmin() || len(got.GetRole()) != 1 {
		t.Fatal("profile update must preserve privileges")
	}
	if got.GetEmail() != "ada@ex.com" {
		t.Fatal("email must not change via own profile")
	}
}

func TestLastAdminProtection(t *testing.T) {
	t.Parallel()
	repo := newMemUsers()
	repo.put(domain.User{UserGeneric: authBaseDomain.NewUserGeneric("a1", "Admin", "a@ex.com", nil, true, true)})
	repo.admins = 1
	audit := &memAudit{}
	svc := services.NewUserService(repo, nil, audit)
	if err := svc.SetAdminFlag("a1", "a1", false); !errors.Is(err, domain.ErrLastAdmin) {
		t.Fatalf("SetAdminFlag err=%v", err)
	}
	if err := svc.ChangeStatus("a1", "a1"); !errors.Is(err, domain.ErrLastAdmin) {
		t.Fatalf("ChangeStatus err=%v", err)
	}
}

func TestAssignRolesAudits(t *testing.T) {
	t.Parallel()
	repo := newMemUsers()
	repo.put(domain.User{UserGeneric: authBaseDomain.NewUserGeneric("u1", "Ada", "ada@ex.com", nil, false, true)})
	audit := &memAudit{}
	svc := services.NewUserService(repo, nil, audit)
	if err := svc.AssignRoles("admin-1", "u1", []string{"role-x"}); err != nil {
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
	if err := boot.Run("Admin", "admin@ex.com", "CorrectHorse1"); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := boot.Run("Admin", "admin2@ex.com", "CorrectHorse1"); !errors.Is(err, domain.ErrAdminExists) {
		t.Fatalf("second: %v", err)
	}
}

func TestAdminBootstrapRejectsWeakPassword(t *testing.T) {
	t.Parallel()
	boot := services.NewAdminBootstrap(newMemUsers())
	if err := boot.Run("Admin", "admin@ex.com", "weak"); !errors.Is(err, domain.ErrWeakPassword) {
		t.Fatalf("err=%v", err)
	}
}
