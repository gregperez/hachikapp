package auditstore_test

import (
	"context"
	"strings"
	"testing"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/WilsonSayago/base-golang/internal/core/domain"
	"github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/postgres/auditstore"
	"github.com/WilsonSayago/base-golang/internal/test/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
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
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return pool
}

func setupAuditSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	stmts := []string{
		`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`,
		`CREATE TABLE IF NOT EXISTS public.audit_events (
			id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
			occurred_at timestamptz NOT NULL DEFAULT now(),
			category varchar(32) NOT NULL,
			action varchar(64) NOT NULL,
			outcome varchar(16) NOT NULL,
			actor_id varchar(64) NOT NULL,
			subject_type varchar(32) NOT NULL DEFAULT '',
			subject_id varchar(64) NOT NULL DEFAULT '',
			request_id varchar(64) NOT NULL DEFAULT '',
			metadata jsonb NOT NULL DEFAULT '{}'::jsonb
		)`,
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `TRUNCATE public.audit_events`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func TestAuditStoreInsertListAndStableOrder(t *testing.T) {
	pool := testPool(t)
	setupAuditSchema(t, pool)
	store := auditstore.New(database.NewPostgresPgxForTest(pool))
	ctx := context.Background()
	now := time.Date(2026, 9, 28, 18, 0, 0, 0, time.UTC)
	same := now
	if err := store.Insert(ctx, domain.AuditEvent{
		OccurredAt: same, Category: domain.AuditCategorySecurity, Action: "login_success",
		Outcome: "success", ActorID: "", SubjectType: domain.AuditSubjectUser, RequestID: "rid-1",
		Metadata: map[string]any{"password": "hunter2hunter2xx", "provider": "google"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Insert(ctx, domain.AuditEvent{
		OccurredAt: same, Category: domain.AuditCategoryPrivilege, Action: "assign_roles",
		Outcome: "success", ActorID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		SubjectType: domain.AuditSubjectUser, SubjectID: "bbbbbbbb-bbbb-cccc-dddd-eeeeeeeeeeee",
	}); err != nil {
		t.Fatal(err)
	}

	page, err := store.List(ctx, domain.AuditListFilter{}, authBaseDomain.PageRequest{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("items=%d", len(page.Items))
	}
	var systemActor string
	for _, item := range page.Items {
		if item.Action == "login_success" {
			systemActor = item.ActorID
		}
	}
	if systemActor != domain.AuditActorSystem {
		t.Fatalf("system actor=%s", systemActor)
	}
	raw, err := pool.Query(ctx, `SELECT metadata::text FROM public.audit_events`)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	for raw.Next() {
		var meta string
		if err := raw.Scan(&meta); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToLower(meta), "password") || strings.Contains(meta, "hunter") {
			t.Fatalf("secret stored: %s", meta)
		}
	}

	first, err := store.List(ctx, domain.AuditListFilter{}, authBaseDomain.PageRequest{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !first.HasNext || len(first.Items) != 1 {
		t.Fatalf("first=%+v", first)
	}
	second, err := store.List(ctx, domain.AuditListFilter{}, authBaseDomain.PageRequest{
		Limit: 1,
		After: &authBaseDomain.CursorKey{CreatedAt: first.Items[0].OccurredAt, ID: first.Items[0].ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.Items[0].ID == first.Items[0].ID {
		t.Fatalf("cursor overlap %+v %+v", first.Items, second.Items)
	}
}

func TestAuditStoreRejectsUpdateAndPurges(t *testing.T) {
	pool := testPool(t)
	setupAuditSchema(t, pool)
	_, err := pool.Exec(context.Background(), `
CREATE OR REPLACE FUNCTION public.reject_audit_event_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'audit_events is append-only'; END; $$`)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS audit_events_no_update ON public.audit_events`)
	if _, err := pool.Exec(context.Background(), `
CREATE TRIGGER audit_events_no_update BEFORE UPDATE ON public.audit_events
FOR EACH ROW EXECUTE FUNCTION public.reject_audit_event_update()`); err != nil {
		t.Fatal(err)
	}
	store := auditstore.NewWithPurgeBatchSize(database.NewPostgresPgxForTest(pool), 1)
	ctx := context.Background()
	old := time.Now().UTC().Add(-48 * time.Hour)
	if err := store.Insert(ctx, domain.AuditEvent{
		OccurredAt: old, Category: domain.AuditCategorySecurity, Action: "login_failure",
		Outcome: "failure", ActorID: "system",
	}); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `UPDATE public.audit_events SET outcome = 'success'`)
	if err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("update err=%v", err)
	}
	deleted, err := store.PurgeBefore(ctx, time.Now().UTC().Add(-24*time.Hour))
	if err != nil || deleted != 1 {
		t.Fatalf("deleted=%d err=%v", deleted, err)
	}
}
