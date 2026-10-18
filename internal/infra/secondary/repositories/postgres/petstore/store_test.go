package petstore_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/database"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories/postgres/petstore"
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
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.pets') IS NOT NULL`).Scan(&exists); err != nil {
		t.Fatalf("check schema: %v", err)
	}
	if !exists {
		t.Skip("public.pets missing: run migrations first")
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
VALUES ('Pet Owner', $1, 'x', true) RETURNING id::text`, email).Scan(&id); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM public.users WHERE id = $1::uuid`, id) })
	return id
}

func details(chip string) domain.PetDetails {
	birth := time.Date(2021, 3, 14, 0, 0, 0, 0, time.UTC)
	weight := 12.5
	return domain.NormalizePetDetails(domain.PetDetails{
		Name:       "Luna",
		Species:    domain.PetSpeciesDog,
		BirthDate:  &birth,
		ChipNumber: chip,
		WeightKg:   &weight,
		Health:     domain.PetHealth{Allergies: []string{"Atún"}},
		Owner:      domain.PetOwnerContact{FullName: "Wilson", Comuna: "Ñuñoa", Phone: "+56 9 1234 5678"},
		PublicVisibility: map[domain.PetPublicField]domain.PetVisibility{
			domain.PetPublicBreed: domain.PetVisibilityAlways,
		},
	})
}

func TestPetStoreCRUDIsOwnerScoped(t *testing.T) {
	pool := testPool(t)
	db := database.NewPostgresPgxForTest(pool)
	store := petstore.New(db)
	ctx := context.Background()
	ownerA := createUser(t, pool, "pet-owner-a@example.test")
	ownerB := createUser(t, pool, "pet-owner-b@example.test")

	created, err := store.Create(ctx, ownerA, details("900000000000001"))
	if err != nil {
		t.Fatal(err)
	}
	if created.Version != 1 || created.CreatedAt.IsZero() || created.HasPhoto {
		t.Fatalf("created=%+v", created)
	}
	if created.WeightKg == nil || *created.WeightKg != 12.5 || domain.FormatPetBirthDate(created.BirthDate) != "2021-03-14" {
		t.Fatalf("typed columns: weight=%v birth=%v", created.WeightKg, created.BirthDate)
	}
	if created.PublicVisibility[domain.PetPublicBreed] != domain.PetVisibilityAlways || created.PublicVisibility[domain.PetPublicPhoto] != domain.PetVisibilityAlways {
		t.Fatalf("visibility=%v", created.PublicVisibility)
	}

	if _, err := store.GetForOwner(ctx, ownerB, created.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("owner B get: %v", err)
	}
	if err := store.Delete(ctx, ownerB, created.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("owner B delete: %v", err)
	}

	next := details("900000000000001")
	next.Breed = "Quiltro"
	next.ChipNumber = ""
	updated, err := store.Update(ctx, ownerA, created.ID, 1, next)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != 2 || updated.Breed != "Quiltro" || updated.ChipNumber != "" || !updated.UpdatedAt.After(created.CreatedAt.Add(-time.Second)) {
		t.Fatalf("updated=%+v", updated)
	}
	if _, err := store.Update(ctx, ownerA, created.ID, 1, next); !errors.Is(err, domain.ErrStaleVersion) {
		t.Fatalf("stale: %v", err)
	}
	if _, err := store.Update(ctx, ownerB, created.ID, 2, next); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("owner B update: %v", err)
	}

	list, err := store.ListByOwner(ctx, ownerA, 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}
	if count, err := store.CountByOwner(ctx, ownerB); err != nil || count != 0 {
		t.Fatalf("count B=%d err=%v", count, err)
	}

	if err := store.Delete(ctx, ownerA, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetForOwner(ctx, ownerA, created.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}

func TestPetStoreChipIsUniquePerAccount(t *testing.T) {
	pool := testPool(t)
	store := petstore.New(database.NewPostgresPgxForTest(pool))
	ctx := context.Background()
	ownerA := createUser(t, pool, "pet-chip-a@example.test")
	ownerB := createUser(t, pool, "pet-chip-b@example.test")

	if _, err := store.Create(ctx, ownerA, details("900000000000002")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(ctx, ownerA, details("900000000000002")); !errors.Is(err, domain.ErrDuplicateChip) {
		t.Fatalf("same account, same chip: %v", err)
	}
	// Another account may register the same chip: a global constraint would let
	// anyone squat a chip and reveal whether it exists.
	if _, err := store.Create(ctx, ownerB, details("900000000000002")); err != nil {
		t.Fatalf("other account must not collide: %v", err)
	}
	if _, err := store.Create(ctx, ownerB, details("")); err != nil {
		t.Fatalf("pets without chip must not collide: %v", err)
	}
	if _, err := store.Create(ctx, ownerB, details("")); err != nil {
		t.Fatalf("second pet without chip: %v", err)
	}
}

func TestPhotoStoreRoundTripAndCascade(t *testing.T) {
	pool := testPool(t)
	db := database.NewPostgresPgxForTest(pool)
	store := petstore.New(db)
	photos := petstore.NewPhotoStore(db)
	ctx := context.Background()
	owner := createUser(t, pool, "pet-photo@example.test")

	pet, err := store.Create(ctx, owner, details(""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := photos.Get(ctx, pet.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing photo: %v", err)
	}
	at := time.Now().UTC().Truncate(time.Microsecond)
	data := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 64)...)
	if err := photos.Put(ctx, pet.ID, domain.PetPhoto{ContentType: "image/png", Data: data, UpdatedAt: at}); err != nil {
		t.Fatal(err)
	}
	if err := photos.Put(ctx, pet.ID, domain.PetPhoto{ContentType: "image/webp", Data: data, UpdatedAt: at}); err != nil {
		t.Fatalf("replace photo: %v", err)
	}
	got, err := photos.Get(ctx, pet.ID)
	if err != nil || got.ContentType != "image/webp" || !bytes.Equal(got.Data, data) {
		t.Fatalf("photo=%+v err=%v", got.ContentType, err)
	}
	marked, err := store.SetPhotoUpdatedAt(ctx, owner, pet.ID, &at)
	if err != nil || !marked.HasPhoto || !marked.PhotoUpdatedAt.Equal(at) {
		t.Fatalf("marked=%+v err=%v", marked, err)
	}
	if err := photos.Put(ctx, pet.ID, domain.PetPhoto{ContentType: "image/gif", Data: data, UpdatedAt: at}); err == nil {
		t.Fatal("database must reject unsupported content types")
	}

	if err := store.Delete(ctx, owner, pet.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := photos.Get(ctx, pet.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("photo must cascade with the pet: %v", err)
	}
	if err := photos.Delete(ctx, pet.ID); err != nil {
		t.Fatalf("delete must be idempotent: %v", err)
	}
}
