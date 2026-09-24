package dao_test

import (
	"context"
	"errors"
	"testing"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v3/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/dto"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/postgres/dao"
	"github.com/WilsonSayago/base-golang/internal/test/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

func openTestDB(t *testing.T) (*database.PostgresPgx, context.Context) {
	t.Helper()
	dsn, err := testdb.RequireURL()
	if err != nil {
		t.Skip(err.Error())
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return database.NewPostgresPgxForTest(pool), ctx
}

func TestSetActiveIdempotent(t *testing.T) {
	db, ctx := openTestDB(t)
	ud := dao.NewUserDaoForTest(db)

	email := "persist-active-" + time.Now().Format("150405.000") + "@example.com"
	u := &dto.UserDTO{Name: "T", Email: email, Password: "hash", IsAdmin: false}
	if err := ud.Save(ctx, u); err != nil {
		t.Fatal(err)
	}
	if err := ud.SetActive(ctx, u.Id, true); err != nil {
		t.Fatal(err)
	}
	if err := ud.SetActive(ctx, u.Id, true); err != nil {
		t.Fatal(err)
	}
	got, err := ud.FindById(ctx, u.Id)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Active {
		t.Fatal("expected active")
	}
}

func TestUpdateMissingUserNotFound(t *testing.T) {
	db, ctx := openTestDB(t)
	ud := dao.NewUserDaoForTest(db)
	err := ud.Update(ctx, dto.UserDTO{
		BaseDTO: dto.BaseDTO{Id: "00000000-0000-0000-0000-000000000099"},
		Name:    "x",
		Email:   "missing@example.com",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestUpdateRolePersistsRoutes(t *testing.T) {
	db, ctx := openTestDB(t)
	rd := dao.NewRoleDaoForTest(db)
	name := "role-" + time.Now().Format("150405.000")
	saved, err := rd.Save(ctx, dto.RoleDTO{
		Name:        name,
		Routes:      []authBaseDomain.RoutePathEnum{authBaseDomain.USERS},
		Permissions: []dto.PermissionDTO{{Entity: "USERS", Read: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	saved.Routes = []authBaseDomain.RoutePathEnum{authBaseDomain.USERS, authBaseDomain.ROLES}
	saved.Permissions = []dto.PermissionDTO{{Entity: "ROLES", Update: true}}
	if err := rd.Update(ctx, saved); err != nil {
		t.Fatal(err)
	}
	got, err := rd.FindById(ctx, saved.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 2 {
		t.Fatalf("routes=%v", got.Routes)
	}
	if len(got.Permissions) != 1 || got.Permissions[0].Entity != "ROLES" {
		t.Fatalf("permissions=%v", got.Permissions)
	}
}

func TestFindAllCanceledContext(t *testing.T) {
	db, _ := openTestDB(t)
	ud := dao.NewUserDaoForTest(db)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ud.FindAll(ctx, 10, 0)
	if err == nil {
		t.Fatal("expected error on canceled context")
	}
}
