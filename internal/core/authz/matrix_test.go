package authz_test

import (
	"testing"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/authz"
)

func TestPermissionMatrix(t *testing.T) {
	t.Parallel()

	guest := authBaseDomain.NewUserGeneric("g1", "Guest", "g@ex.com", []authBaseDomain.Role{
		{Base: authBaseDomain.Base{Id: "r-guest", Active: true}, Name: "guest", Permissions: nil},
	}, false, true)

	user := authBaseDomain.NewUserGeneric("u1", "User", "u@ex.com", []authBaseDomain.Role{
		{Base: authBaseDomain.Base{Id: "r-user", Active: true}, Name: "user", Permissions: nil},
	}, false, true)

	admin := authBaseDomain.NewUserGeneric("a1", "Admin", "a@ex.com", []authBaseDomain.Role{
		{
			Base: authBaseDomain.Base{Id: "r-admin", Active: true},
			Name: "admin",
			Permissions: []authBaseDomain.Permission{
				{Entity: authz.EntityUsers, Create: true, Read: true, Update: true, Delete: true},
				{Entity: authz.EntityRoles, Create: true, Read: true, Update: true, Delete: true},
				{Entity: authz.EntityUsersPrivileges, Update: true},
				{Entity: authz.EntityAuditEvents, Read: true},
				{Entity: authz.EntitySettings, Read: true, Update: true},
			},
		},
	}, false, true)

	// is_admin bypasses permission checks in authBase Authorization.IsAuthorized;
	// this matrix covers role-permission paths (least privilege seeds).
	cases := []struct {
		name   string
		subj   authBaseDomain.UserGeneric
		entity string
		op     authBaseDomain.OperationEnum
		want   bool
	}{
		{"guest roles read", guest, authz.EntityRoles, authz.Read, false},
		{"guest roles create", guest, authz.EntityRoles, authz.Create, false},
		{"guest users read", guest, authz.EntityUsers, authz.Read, false},
		{"user roles crud", user, authz.EntityRoles, authz.Update, false},
		{"user users update", user, authz.EntityUsers, authz.Update, false},
		{"user privileges", user, authz.EntityUsersPrivileges, authz.Update, false},
		{"admin roles read", admin, authz.EntityRoles, authz.Read, true},
		{"admin roles create", admin, authz.EntityRoles, authz.Create, true},
		{"admin users create", admin, authz.EntityUsers, authz.Create, true},
		{"admin privileges", admin, authz.EntityUsersPrivileges, authz.Update, true},
		{"admin audit read", admin, authz.EntityAuditEvents, authz.Read, true},
		{"user audit read", user, authz.EntityAuditEvents, authz.Read, false},
		{"admin settings read", admin, authz.EntitySettings, authz.Read, true},
		{"admin settings update", admin, authz.EntitySettings, authz.Update, true},
		{"user settings update", user, authz.EntitySettings, authz.Update, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.subj.HasPermission(tc.entity, tc.op)
			if got != tc.want {
				t.Fatalf("HasPermission(%s,%s)=%v want %v", tc.entity, tc.op, got, tc.want)
			}
		})
	}
}
