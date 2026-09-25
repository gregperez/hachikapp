package dao_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
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
	_, _, err := ud.FindAll(ctx, authBaseDomain.PageRequest{Limit: 10})
	if err == nil {
		t.Fatal("expected error on canceled context")
	}
}

func TestUserFindAllUsesCreatedAtIDCursor(t *testing.T) {
	db, ctx := openTestDB(t)
	ud := dao.NewUserDaoForTest(db)
	createdAt := time.Date(1900, time.January, 1, 0, 0, 0, 0, time.UTC)
	ids := []string{
		"f0000000-0000-0000-0000-000000000001",
		"f0000000-0000-0000-0000-000000000002",
		"f0000000-0000-0000-0000-000000000003",
	}
	for i, id := range ids {
		if _, err := db.Exec(ctx, `
INSERT INTO public.users (id, name, email, password, created_at)
VALUES ($1, $2, $3, 'hash', $4)
ON CONFLICT (id) DO NOTHING`,
			id,
			"cursor-user",
			fmt.Sprintf("cursor-user-%d@example.com", i),
			createdAt,
		); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), `DELETE FROM public.users WHERE id = ANY($1::uuid[])`, ids)
	})

	first, hasNext, err := ud.FindAll(ctx, authBaseDomain.PageRequest{
		Limit: 2,
		After: &authBaseDomain.CursorKey{
			CreatedAt: time.Date(1901, time.January, 1, 0, 0, 0, 0, time.UTC),
			ID:        "ffffffff-ffff-ffff-ffff-ffffffffffff",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasNext || len(first) != 2 || first[0].Id != ids[2] || first[1].Id != ids[1] {
		t.Fatalf("first page ids=%v hasNext=%v", dtoIDs(first), hasNext)
	}

	second, hasNext, err := ud.FindAll(ctx, authBaseDomain.PageRequest{
		Limit: 2,
		After: &authBaseDomain.CursorKey{CreatedAt: first[1].CreatedAt, ID: first[1].Id},
	})
	if err != nil {
		t.Fatal(err)
	}
	if hasNext || len(second) == 0 || second[0].Id != ids[0] {
		t.Fatalf("second page ids=%v hasNext=%v", dtoIDs(second), hasNext)
	}
}

func TestRoleFindAllUsesCreatedAtIDCursor(t *testing.T) {
	db, ctx := openTestDB(t)
	rd := dao.NewRoleDaoForTest(db)
	createdAt := time.Date(1900, time.January, 2, 0, 0, 0, 0, time.UTC)
	ids := []string{
		"e0000000-0000-0000-0000-000000000001",
		"e0000000-0000-0000-0000-000000000002",
		"e0000000-0000-0000-0000-000000000003",
	}
	for i, id := range ids {
		if _, err := db.Exec(ctx, `
INSERT INTO public.roles (id, name, created_at)
VALUES ($1, $2, $3)
ON CONFLICT (id) DO NOTHING`,
			id,
			fmt.Sprintf("cursor-role-%d", i),
			createdAt,
		); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), `DELETE FROM public.roles WHERE id = ANY($1::uuid[])`, ids)
	})

	first, hasNext, err := rd.FindAll(ctx, authBaseDomain.PageRequest{
		Limit: 2,
		After: &authBaseDomain.CursorKey{
			CreatedAt: time.Date(1901, time.January, 1, 0, 0, 0, 0, time.UTC),
			ID:        "ffffffff-ffff-ffff-ffff-ffffffffffff",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasNext || len(first) != 2 || first[0].Id != ids[2] || first[1].Id != ids[1] {
		t.Fatalf("first page ids=%v hasNext=%v", roleDTOIDs(first), hasNext)
	}

	second, hasNext, err := rd.FindAll(ctx, authBaseDomain.PageRequest{
		Limit: 2,
		After: &authBaseDomain.CursorKey{CreatedAt: first[1].CreatedAt, ID: first[1].Id},
	})
	if err != nil {
		t.Fatal(err)
	}
	if hasNext || len(second) == 0 || second[0].Id != ids[0] {
		t.Fatalf("second page ids=%v hasNext=%v", roleDTOIDs(second), hasNext)
	}
}

func dtoIDs(items []dto.UserDTO) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Id)
	}
	return ids
}

func roleDTOIDs(items []dto.RoleDTO) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Id)
	}
	return ids
}
