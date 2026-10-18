package services

import (
	"context"
	"fmt"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	authBasePorts "github.com/WilsonSayago/authBase/v4/core/port"
	"github.com/WilsonSayago/base-golang/internal/core"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/ports"
)

type UserService struct {
	userRepository ports.UserPort
	validation     authBasePorts.ValidationPort
	audit          ports.PrivilegeAuditor
	passwords      domain.PasswordBlocklist
}

func GetUserServiceInstance(
	userRepository ports.UserPort,
	validation authBasePorts.ValidationPort,
	audit ports.PrivilegeAuditor,
	passwords ...domain.PasswordBlocklist,
) core.UserUseCase {
	return NewUserService(userRepository, validation, audit, passwords...)
}

// NewUserService constructs a UserService without process-wide state.
func NewUserService(
	userRepository ports.UserPort,
	validation authBasePorts.ValidationPort,
	audit ports.PrivilegeAuditor,
	passwords ...domain.PasswordBlocklist,
) *UserService {
	if audit == nil {
		audit = ports.NoopAuditor{}
	}
	var passwordBlocklist domain.PasswordBlocklist
	if len(passwords) > 0 {
		passwordBlocklist = passwords[0]
	}
	return &UserService{
		userRepository: userRepository,
		validation:     validation,
		audit:          audit,
		passwords:      passwordBlocklist,
	}
}

func (s *UserService) FindByID(ctx context.Context, id string) (domain.User, error) {
	return s.userRepository.FindByID(ctx, id)
}

func (s *UserService) FindAll(ctx context.Context, request authBaseDomain.PageRequest) (authBaseDomain.Page[domain.User], error) {
	return s.userRepository.FindAll(ctx, request)
}

func (s *UserService) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	return s.userRepository.FindByEmail(ctx, email)
}

func (s *UserService) FindFullById(ctx context.Context, id string) (domain.User, error) {
	return s.userRepository.FindFullById(ctx, id)
}

func (s *UserService) Create(ctx context.Context, user domain.User) error {
	if err := domain.ValidatePasswordContext(
		ctx,
		user.PlainPassword(),
		s.passwords,
		user.GetName(),
		user.GetEmail(),
	); err != nil {
		return err
	}
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
	return s.userRepository.Save(ctx, clean)
}

func (s *UserService) FindById(ctx context.Context, id string) (domain.User, error) {
	return s.userRepository.FindById(ctx, id)
}

func (s *UserService) UpdateOwnProfile(ctx context.Context, actorID, userID, name string) error {
	if actorID == "" || userID == "" || actorID != userID {
		return domain.ErrForbidden
	}
	existing, err := s.userRepository.FindFullById(ctx, userID)
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
	return s.userRepository.Update(ctx, updated)
}

func (s *UserService) AdminUpdateUser(ctx context.Context, userID, name, email string) error {
	existing, err := s.userRepository.FindFullById(ctx, userID)
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
	return s.userRepository.Update(ctx, updated)
}

func (s *UserService) AssignRoles(ctx context.Context, actorID, userID string, roleIDs []string) error {
	if len(roleIDs) == 0 {
		return fmt.Errorf("errors.users.roles-required")
	}
	existing, err := s.userRepository.FindFullById(ctx, userID)
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
	if err := s.userRepository.Update(ctx, updated); err != nil {
		return err
	}
	s.audit.PrivilegeChange(ctx, "assign_roles", actorID, userID, roleIDs, nil)
	return nil
}

func (s *UserService) SetAdminFlag(ctx context.Context, actorID, userID string, isAdmin bool) error {
	existing, err := s.userRepository.FindFullById(ctx, userID)
	if err != nil {
		return err
	}
	if existing.GetIsAdmin() == isAdmin {
		return nil
	}
	if existing.GetIsAdmin() && !isAdmin {
		if err := s.userRepository.DemoteAdminIfNotLast(ctx, userID); err != nil {
			return err
		}
		flag := false
		s.audit.PrivilegeChange(ctx, "set_admin_flag", actorID, userID, nil, &flag)
		return nil
	}
	updated := domain.User{
		UserGeneric: authBaseDomain.NewUserGeneric(
			existing.GetId(),
			existing.GetName(),
			existing.GetEmail(),
			existing.GetRole(),
			true,
			existing.GetActive(),
		),
	}
	if err := s.userRepository.Update(ctx, updated); err != nil {
		return err
	}
	flag := true
	s.audit.PrivilegeChange(ctx, "set_admin_flag", actorID, userID, nil, &flag)
	return nil
}

func (s *UserService) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	existing, err := s.userRepository.FindFullById(ctx, userID)
	if err != nil {
		return err
	}
	if err := domain.ValidatePasswordContext(
		ctx,
		newPassword,
		s.passwords,
		existing.GetName(),
		existing.GetEmail(),
	); err != nil {
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
	return s.userRepository.UpdatePasswordAndRevokeSessions(ctx, userID, hash)
}

func (s *UserService) SetActive(ctx context.Context, actorID, userID string, active bool) error {
	existing, err := s.userRepository.FindFullById(ctx, userID)
	if err != nil {
		return err
	}
	if existing.GetActive() == active {
		return nil
	}
	if existing.GetIsAdmin() && existing.GetActive() && !active {
		if err := s.userRepository.DeactivateAdminAndRevokeSessions(ctx, userID); err != nil {
			return err
		}
		s.audit.PrivilegeChange(ctx, "set_active", actorID, userID, nil, nil)
		return nil
	}
	if !active {
		if err := s.userRepository.DeactivateAndRevokeSessions(ctx, userID); err != nil {
			return err
		}
		s.audit.PrivilegeChange(ctx, "set_active", actorID, userID, nil, nil)
		return nil
	}
	if err := s.userRepository.SetActive(ctx, userID, active); err != nil {
		return err
	}
	s.audit.PrivilegeChange(ctx, "set_active", actorID, userID, nil, nil)
	return nil
}
