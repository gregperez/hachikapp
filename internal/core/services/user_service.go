package services

import (
	"context"
	"fmt"

	authBaseDomain "github.com/WilsonSayago/authBase/v3/core/domain"
	authBasePorts "github.com/WilsonSayago/authBase/v3/core/port"
	"github.com/WilsonSayago/base-golang/internal/core"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/ports"
	initModules "github.com/WilsonSayago/initModules/v2"
)

type UserService struct {
	userRepository ports.UserPort
	validation     authBasePorts.ValidationPort
	audit          ports.PrivilegeAuditor
}

func GetUserServiceInstance(
	userRepository ports.UserPort,
	validation authBasePorts.ValidationPort,
	audit ports.PrivilegeAuditor,
) core.UserUseCase {
	return initModules.Once(func() *UserService {
		if audit == nil {
			audit = ports.NoopAuditor{}
		}
		return &UserService{
			userRepository: userRepository,
			validation:     validation,
			audit:          audit,
		}
	})
}

// NewUserService constructs a UserService without the process-wide Once (tests).
func NewUserService(
	userRepository ports.UserPort,
	validation authBasePorts.ValidationPort,
	audit ports.PrivilegeAuditor,
) *UserService {
	if audit == nil {
		audit = ports.NoopAuditor{}
	}
	return &UserService{
		userRepository: userRepository,
		validation:     validation,
		audit:          audit,
	}
}

func (s *UserService) FindByID(ctx context.Context, id string) (domain.User, error) {
	return s.userRepository.FindByID(ctx, id)
}

func (s *UserService) FindAll(pageSize, offset int) ([]domain.User, int, error) {
	return s.userRepository.FindAll(pageSize, offset)
}

func (s *UserService) FindByEmail(email string) (domain.User, error) {
	return s.userRepository.FindByEmail(email)
}

func (s *UserService) FindFullById(id string) (domain.User, error) {
	return s.userRepository.FindFullById(id)
}

func (s *UserService) Create(user domain.User) error {
	if err := domain.ValidatePassword(user.PlainPassword()); err != nil {
		return err
	}
	// Strip privileges: mass-assignment of isAdmin/roles is never accepted here.
	clean := domain.WithPlainPassword(domain.User{
		UserGeneric: authBaseDomain.NewUserGeneric(
			"",
			user.GetName(),
			user.GetEmail(),
			nil,
			false,
			true,
		),
	}, user.PlainPassword())
	return s.userRepository.Save(clean)
}

func (s *UserService) FindById(id string) (domain.User, error) {
	return s.userRepository.FindById(id)
}

func (s *UserService) UpdateOwnProfile(actorID, userID, name string) error {
	if actorID == "" || userID == "" || actorID != userID {
		return domain.ErrForbidden
	}
	existing, err := s.userRepository.FindFullById(userID)
	if err != nil {
		return err
	}
	updated := domain.User{
		UserGeneric: authBaseDomain.NewUserGeneric(
			existing.GetId(),
			name,
			existing.GetEmail(),
			existing.GetRole(),
			existing.GetIsAdmin(),
			existing.GetActive(),
		),
	}
	return s.userRepository.Update(updated)
}

func (s *UserService) AdminUpdateUser(userID, name, email string) error {
	existing, err := s.userRepository.FindFullById(userID)
	if err != nil {
		return err
	}
	updated := domain.User{
		UserGeneric: authBaseDomain.NewUserGeneric(
			existing.GetId(),
			name,
			email,
			existing.GetRole(),
			existing.GetIsAdmin(),
			existing.GetActive(),
		),
	}
	return s.userRepository.Update(updated)
}

func (s *UserService) AssignRoles(actorID, userID string, roleIDs []string) error {
	if len(roleIDs) == 0 {
		return fmt.Errorf("errors.users.roles-required")
	}
	existing, err := s.userRepository.FindFullById(userID)
	if err != nil {
		return err
	}
	roles := make([]authBaseDomain.Role, 0, len(roleIDs))
	for _, id := range roleIDs {
		roles = append(roles, authBaseDomain.Role{Base: authBaseDomain.Base{Id: id, Active: true}})
	}
	updated := domain.User{
		UserGeneric: authBaseDomain.NewUserGeneric(
			existing.GetId(),
			existing.GetName(),
			existing.GetEmail(),
			roles,
			existing.GetIsAdmin(),
			existing.GetActive(),
		),
	}
	if err := s.userRepository.Update(updated); err != nil {
		return err
	}
	s.audit.PrivilegeChange("assign_roles", actorID, userID, roleIDs, nil)
	return nil
}

func (s *UserService) SetAdminFlag(actorID, userID string, isAdmin bool) error {
	existing, err := s.userRepository.FindFullById(userID)
	if err != nil {
		return err
	}
	if existing.GetIsAdmin() && !isAdmin {
		if err := s.guardLastAdmin(); err != nil {
			return err
		}
	}
	updated := domain.User{
		UserGeneric: authBaseDomain.NewUserGeneric(
			existing.GetId(),
			existing.GetName(),
			existing.GetEmail(),
			existing.GetRole(),
			isAdmin,
			existing.GetActive(),
		),
	}
	if err := s.userRepository.Update(updated); err != nil {
		return err
	}
	flag := isAdmin
	s.audit.PrivilegeChange("set_admin_flag", actorID, userID, nil, &flag)
	return nil
}

func (s *UserService) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := domain.ValidatePassword(newPassword); err != nil {
		return err
	}
	existing, err := s.userRepository.FindFullById(userID)
	if err != nil {
		return err
	}
	cred, err := s.userRepository.FindCredentialsByUsername(ctx, existing.GetEmail())
	if err != nil {
		return err
	}
	if s.validation == nil || !s.validation.CheckPassword(cred.PasswordHash, currentPassword) {
		return domain.ErrForbidden
	}
	hash, err := s.validation.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("errors.users.hash-password")
	}
	return s.userRepository.UpdatePassword(userID, hash)
}

func (s *UserService) ChangeStatus(actorID, userID string) error {
	existing, err := s.userRepository.FindFullById(userID)
	if err != nil {
		return err
	}
	// Toggle will deactivate an active admin — protect the last one.
	if existing.GetIsAdmin() && existing.GetActive() {
		if err := s.guardLastAdmin(); err != nil {
			return err
		}
	}
	if err := s.userRepository.ChangeStatus(userID); err != nil {
		return err
	}
	s.audit.PrivilegeChange("change_status", actorID, userID, nil, nil)
	return nil
}

func (s *UserService) guardLastAdmin() error {
	n, err := s.userRepository.CountAdmins()
	if err != nil {
		return err
	}
	if n <= 1 {
		return domain.ErrLastAdmin
	}
	return nil
}
