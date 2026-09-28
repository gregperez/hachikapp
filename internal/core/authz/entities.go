package authz

import authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"

// Resource entities used by HTTP PoliciesGuard and seeds.
const (
	EntityUsers           = "USERS"
	EntityRoles           = "ROLES"
	EntityUsersPrivileges = "USERS_PRIVILEGES" // assign roles / is_admin
	EntityAuditEvents     = "AUDIT_EVENTS"
	EntitySettings        = "SETTINGS"
)

// Re-export operations for controllers without scattering string literals.
const (
	Create = authBaseDomain.CREATE
	Read   = authBaseDomain.READ
	Update = authBaseDomain.UPDATE
	Delete = authBaseDomain.DELETE
)
