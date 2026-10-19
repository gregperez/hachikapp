package tagstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/database"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories/postgres/petstore"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories/postgres/tagstore"
	"github.com/gregperez/hachikapp-backend/internal/test/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests expect a migrated DATABASE_TEST_URL (make test-db runs
// migrate-verify first) and skip otherwise.
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
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.pet_tags') IS NOT NULL`).Scan(&exists); err != nil {
		t.Fatalf("check schema: %v", err)
	}
	if !exists {
		t.Skip("public.pet_tags missing: run migrations first")
	}
	return pool
}

func createUser(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DELETE FROM public.users WHERE email = $1`, email); err != nil {
		t.Fatalf("cleanup user: %v", err)
	}
	var id string
	if err := pool.QueryRow(ctx, `INSERT INTO public.users (name, email, password, active)
VALUES ('Tag Owner', $1, 'x', true) RETURNING id::text`, email).Scan(&id); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		// Batches created by this user cascade to their tags before the user goes.
		_, _ = pool.Exec(bg, `DELETE FROM public.tag_batches WHERE created_by = $1::uuid`, id)
		_, _ = pool.Exec(bg, `DELETE FROM public.pet_tags WHERE owner_id = $1::uuid`, id)
		_, _ = pool.Exec(bg, `DELETE FROM public.users WHERE id = $1::uuid`, id)
	})
	return id
}

func createPet(t *testing.T, pets *petstore.Store, ownerID, name string) string {
	t.Helper()
	pet, err := pets.Create(context.Background(), ownerID, domain.NormalizePetDetails(domain.PetDetails{
		Name:    name,
		Species: domain.PetSpeciesDog,
		Owner:   domain.PetOwnerContact{FullName: "Wilson", Phone: "+56 9 1234 5678"},
	}))
	if err != nil {
		t.Fatalf("create pet: %v", err)
	}
	return pet.ID
}

func newCodes(prefix string, n int) []string {
	codes := make([]string, n)
	for i := range codes {
		codes[i] = prefix + string(domain.TagCodeAlphabet[i/32]) + string(domain.TagCodeAlphabet[i%32])
	}
	return codes
}

func TestTagStoreBatchAndActivation(t *testing.T) {
	pool := testPool(t)
	db := database.NewPostgresPgxForTest(pool)
	store := tagstore.New(db)
	pets := petstore.New(db)
	ctx := context.Background()
	ownerA := createUser(t, pool, "tag-owner-a@example.test")
	petA := createPet(t, pets, ownerA, "Luna")
	at := time.Now().UTC().Truncate(time.Second)

	codes := newCodes("TAGBATCH", 3)
	batch, err := store.CreateBatch(ctx, domain.TagBatch{Label: "  Lote 1  ", Quantity: 3, CreatedBy: ownerA}, codes)
	if err != nil {
		t.Fatal(err)
	}
	if batch.ID == "" || batch.Label != "Lote 1" || batch.Quantity != 3 || batch.CreatedAt.IsZero() {
		t.Fatalf("batch=%+v", batch)
	}
	tags, err := store.BatchTags(ctx, batch.ID)
	if err != nil || len(tags) != 3 {
		t.Fatalf("batch tags=%v err=%v", tags, err)
	}
	for _, tag := range tags {
		if tag.Kind != domain.TagKindPhysical || tag.Status != domain.TagStatusUnassigned || tag.BatchID != batch.ID || tag.OwnerID != "" || tag.PetID != "" {
			t.Fatalf("tag=%+v", tag)
		}
	}
	page, err := store.ListBatches(ctx, authBaseDomain.PageRequest{Limit: 100})
	if err != nil || len(page.Items) == 0 {
		t.Fatalf("batches=%+v err=%v", page, err)
	}
	if page.Items[0].ID != batch.ID {
		t.Fatalf("newest batch first: got %s want %s", page.Items[0].ID, batch.ID)
	}
	first, err := store.ListBatches(ctx, authBaseDomain.PageRequest{Limit: 1})
	if err != nil || len(first.Items) != 1 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	if len(page.Items) > 1 {
		if !first.HasNext {
			t.Fatal("expected HasNext")
		}
		next, err := store.ListBatches(ctx, authBaseDomain.PageRequest{Limit: 1, After: &authBaseDomain.CursorKey{CreatedAt: first.Items[0].CreatedAt, ID: first.Items[0].ID}})
		if err != nil || len(next.Items) != 1 || next.Items[0].ID == first.Items[0].ID {
			t.Fatalf("next=%+v err=%v", next, err)
		}
	}

	got, err := store.GetByCode(ctx, codes[0])
	if err != nil || got.Code != codes[0] || got.Status != domain.TagStatusUnassigned {
		t.Fatalf("get=%+v err=%v", got, err)
	}
	if _, err := store.GetByCode(ctx, "ZZZZZZZZZZ"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown code: %v", err)
	}

	active, err := store.Activate(ctx, codes[0], ownerA, petA, at)
	if err != nil {
		t.Fatal(err)
	}
	if active.Status != domain.TagStatusActive || active.PetID != petA || active.OwnerID != ownerA || active.ActivatedAt == nil || active.DisabledAt != nil {
		t.Fatalf("active=%+v", active)
	}
	if _, err := store.Activate(ctx, codes[0], ownerA, petA, at); !errors.Is(err, domain.ErrTagNotAssignable) {
		t.Fatalf("second activation: %v", err)
	}
	list, err := store.ListByPet(ctx, ownerA, petA)
	if err != nil || len(list) != 1 || list[0].Code != codes[0] {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	if n, err := store.CountActiveByPet(ctx, petA); err != nil || n != 1 {
		t.Fatalf("count=%d err=%v", n, err)
	}
}

func TestTagStoreDuplicateCode(t *testing.T) {
	pool := testPool(t)
	db := database.NewPostgresPgxForTest(pool)
	store := tagstore.New(db)
	pets := petstore.New(db)
	ctx := context.Background()
	ownerA := createUser(t, pool, "tag-dup-a@example.test")
	petA := createPet(t, pets, ownerA, "Luna")

	codes := newCodes("TAGDPAB0", 2)
	if _, err := store.CreateBatch(ctx, domain.TagBatch{Label: "Dup", Quantity: 2, CreatedBy: ownerA}, codes); err != nil {
		t.Fatal(err)
	}
	// A second batch reusing one code must fail and leave nothing behind.
	fresh := newCodes("TAGDPFR0", 1)
	_, err := store.CreateBatch(ctx, domain.TagBatch{Label: "Dup 2", Quantity: 2, CreatedBy: ownerA}, []string{fresh[0], codes[1]})
	if !errors.Is(err, domain.ErrDuplicateTagCode) {
		t.Fatalf("duplicate batch: %v", err)
	}
	if _, err := store.GetByCode(ctx, fresh[0]); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("batch not rolled back: %v", err)
	}
	var leaked int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.tag_batches WHERE label = 'Dup 2' AND created_by = $1::uuid`, ownerA).Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("leaked batches=%d err=%v", leaked, err)
	}
	if _, err := store.CreateDigital(ctx, ownerA, petA, codes[0], time.Now()); !errors.Is(err, domain.ErrDuplicateTagCode) {
		t.Fatalf("duplicate digital: %v", err)
	}
	digital, err := store.CreateDigital(ctx, ownerA, petA, "DGTAAB0001", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if digital.Kind != domain.TagKindDigital || digital.Status != domain.TagStatusActive || digital.BatchID != "" || digital.PetID != petA || digital.OwnerID != ownerA {
		t.Fatalf("digital=%+v", digital)
	}
}

func TestTagStoreDisableIsOwnerScoped(t *testing.T) {
	pool := testPool(t)
	db := database.NewPostgresPgxForTest(pool)
	store := tagstore.New(db)
	pets := petstore.New(db)
	ctx := context.Background()
	ownerA := createUser(t, pool, "tag-dis-a@example.test")
	ownerB := createUser(t, pool, "tag-dis-b@example.test")
	petA := createPet(t, pets, ownerA, "Luna")
	at := time.Now().UTC()

	tag, err := store.CreateDigital(ctx, ownerA, petA, "DSBAAB0001", at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Disable(ctx, ownerB, petA, tag.ID, at); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("owner B disable: %v", err)
	}
	if list, err := store.ListByPet(ctx, ownerB, petA); err != nil || len(list) != 0 {
		t.Fatalf("owner B list=%v err=%v", list, err)
	}
	disabled, err := store.Disable(ctx, ownerA, petA, tag.ID, at)
	if err != nil {
		t.Fatal(err)
	}
	if disabled.Status != domain.TagStatusDisabled || disabled.DisabledAt == nil || disabled.PetID != petA {
		t.Fatalf("disabled=%+v", disabled)
	}
	if _, err := store.Disable(ctx, ownerA, petA, tag.ID, at); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("second disable: %v", err)
	}
	if n, err := store.CountActiveByPet(ctx, petA); err != nil || n != 0 {
		t.Fatalf("count=%d err=%v", n, err)
	}
}

func TestTagStorePetDeletionReleasesTag(t *testing.T) {
	pool := testPool(t)
	db := database.NewPostgresPgxForTest(pool)
	store := tagstore.New(db)
	pets := petstore.New(db)
	ctx := context.Background()
	ownerA := createUser(t, pool, "tag-rel-a@example.test")
	ownerB := createUser(t, pool, "tag-rel-b@example.test")
	petA := createPet(t, pets, ownerA, "Luna")
	petA2 := createPet(t, pets, ownerA, "Nube")
	at := time.Now().UTC()

	codes := newCodes("TAGRESAB", 2)
	if _, err := store.CreateBatch(ctx, domain.TagBatch{Label: "Release", Quantity: 2, CreatedBy: ownerA}, codes); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Activate(ctx, codes[0], ownerA, petA, at); err != nil {
		t.Fatal(err)
	}
	// A disabled tag is released too.
	disabledTag, err := store.Activate(ctx, codes[1], ownerA, petA, at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Disable(ctx, ownerA, petA, disabledTag.ID, at); err != nil {
		t.Fatal(err)
	}

	if err := pets.Delete(ctx, ownerA, petA); err != nil {
		t.Fatal(err)
	}
	for _, code := range codes {
		released, err := store.GetByCode(ctx, code)
		if err != nil {
			t.Fatal(err)
		}
		if released.Status != domain.TagStatusUnassigned || released.PetID != "" || released.OwnerID != ownerA || released.ActivatedAt != nil || released.DisabledAt != nil {
			t.Fatalf("released=%+v", released)
		}
	}
	if _, err := store.Activate(ctx, codes[0], ownerB, petA2, at); !errors.Is(err, domain.ErrTagNotAssignable) {
		t.Fatalf("owner B activation: %v", err)
	}
	again, err := store.Activate(ctx, codes[0], ownerA, petA2, at)
	if err != nil {
		t.Fatal(err)
	}
	if again.Status != domain.TagStatusActive || again.PetID != petA2 {
		t.Fatalf("again=%+v", again)
	}
}
