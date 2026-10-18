package dao_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/WilsonSayago/authBase/v4/infra/secundary"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/database"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories/postgres/dao"
	"github.com/gregperez/hachikapp-backend/internal/test/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

func openAdminTestDB(t *testing.T) (*pgxpool.Pool, *database.PostgresPgx, context.Context) {
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
	return pool, database.NewPostgresPgxForTest(pool), ctx
}

func clearAdmins(t *testing.T, pool *pgxpool.Pool, ctx context.Context) {
	t.Helper()
	if _, err := pool.Exec(ctx, `UPDATE public.users SET is_admin = false`); err != nil {
		t.Fatal(err)
	}
}

func TestSaveFirstAdminConcurrentExactlyOne(t *testing.T) {
	pool, pg, ctx := openAdminTestDB(t)
	clearAdmins(t, pool, ctx)

	userDao := dao.NewUserDaoForTest(pg)
	repo := repositories.NewUserRepositoryForTest(
		userDao,
		dao.NewRoleDaoForTest(pg),
		dao.NewUserRoleDaoForTest(pg),
		secundary.NewValidationService(),
		nil,
	)

	email := fmt.Sprintf("first-admin-%d@example.com", time.Now().UnixNano())
	var okCount atomic.Int32
	var wg sync.WaitGroup
	const n = 8
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			base := authBaseDomain.NewUserGeneric("", "Admin", email, nil, true, true)
			u := domain.WithPlainPassword(domain.User{UserGeneric: base}, "CorrectHorseBattery1!")
			err := repo.SaveFirstAdmin(ctx, u)
			switch {
			case err == nil:
				okCount.Add(1)
			case errors.Is(err, domain.ErrAdminExists), errors.Is(err, domain.ErrConflict):
				// expected losers
			default:
				t.Errorf("goroutine %d: unexpected err=%v", i, err)
			}
		}()
	}
	wg.Wait()

	if got := okCount.Load(); got != 1 {
		t.Fatalf("winners=%d want 1", got)
	}
	admins, err := userDao.CountAdmins(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if admins != 1 {
		t.Fatalf("admin count=%d want 1", admins)
	}
}

func TestDemoteAdminConcurrentKeepsAtLeastOne(t *testing.T) {
	pool, pg, ctx := openAdminTestDB(t)
	clearAdmins(t, pool, ctx)
	userDao := dao.NewUserDaoForTest(pg)

	ids := make([]string, 2)
	for i := range ids {
		email := fmt.Sprintf("demote-%d-%d@example.com", time.Now().UnixNano(), i)
		if err := pool.QueryRow(ctx,
			`INSERT INTO public.users (name, email, password, is_admin, active) VALUES ('A',$1,'hash',true,true) RETURNING id`,
			email,
		).Scan(&ids[i]); err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		i := i
		go func() {
			defer wg.Done()
			errs[i] = userDao.DemoteAdminIfNotLast(ctx, ids[i])
		}()
	}
	wg.Wait()

	admins, err := userDao.CountAdmins(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if admins < 1 {
		t.Fatalf("admins left=%d errs=%v", admins, errs)
	}
	sawLast := false
	sawOK := false
	for _, e := range errs {
		switch {
		case e == nil:
			sawOK = true
		case errors.Is(e, domain.ErrLastAdmin):
			sawLast = true
		default:
			t.Fatalf("unexpected demote err: %v", e)
		}
	}
	if !sawOK || !sawLast {
		t.Fatalf("expected one success and one ErrLastAdmin; errs=%v", errs)
	}
}
