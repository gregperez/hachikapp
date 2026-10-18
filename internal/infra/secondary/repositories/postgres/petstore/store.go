// Package petstore implements the pet record and pet photo ports on PostgreSQL.
package petstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const petColumns = `
id::text, owner_id::text, name, species, breed, color, sex, birth_date, chip_number,
weight_kg::float8, size, distinctive_features, allergies, surgeries, special_conditions,
temperament, owner_full_name, owner_address, owner_comuna, owner_phone, owner_phone_alt,
second_owner_full_name, second_owner_phone, public_visibility, photo_updated_at,
version, created_at, updated_at`

const chipUniqueIndex = "pets_chip_number_uq"

// Store persists pet records.
type Store struct {
	db *database.PostgresPgx
}

func New(db *database.PostgresPgx) *Store {
	return &Store{db: db}
}

var _ ports.PetPort = (*Store)(nil)

func (s *Store) ListByOwner(ctx context.Context, ownerID string, limit int) ([]domain.Pet, error) {
	if limit <= 0 || limit > domain.MaxPetsPerOwner {
		limit = domain.MaxPetsPerOwner
	}
	rows, err := s.db.Query(ctx, `SELECT `+petColumns+`
FROM public.pets
WHERE owner_id = $1::uuid
ORDER BY created_at DESC, id DESC
LIMIT $2`, ownerID, limit)
	if err != nil {
		return nil, fmt.Errorf("list pets: %w", err)
	}
	defer rows.Close()
	pets := make([]domain.Pet, 0)
	for rows.Next() {
		pet, err := scanPet(rows)
		if err != nil {
			return nil, fmt.Errorf("scan pet: %w", err)
		}
		pets = append(pets, pet)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list pets: %w", err)
	}
	return pets, nil
}

func (s *Store) CountByOwner(ctx context.Context, ownerID string) (int, error) {
	var count int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM public.pets WHERE owner_id = $1::uuid`, ownerID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count pets: %w", err)
	}
	return count, nil
}

func (s *Store) GetForOwner(ctx context.Context, ownerID, petID string) (domain.Pet, error) {
	row := s.db.QueryRow(ctx, `SELECT `+petColumns+`
FROM public.pets
WHERE id = $1::uuid AND owner_id = $2::uuid`, petID, ownerID)
	return singlePet(row, "get pet")
}

func (s *Store) Create(ctx context.Context, ownerID string, details domain.PetDetails) (domain.Pet, error) {
	args, err := detailArgs(details)
	if err != nil {
		return domain.Pet{}, err
	}
	row := s.db.QueryRow(ctx, `INSERT INTO public.pets (
    owner_id, name, species, breed, color, sex, birth_date, chip_number, weight_kg, size,
    distinctive_features, allergies, surgeries, special_conditions, temperament,
    owner_full_name, owner_address, owner_comuna, owner_phone, owner_phone_alt,
    second_owner_full_name, second_owner_phone, public_visibility
) VALUES (
    $1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15,
    $16, $17, $18, $19, $20,
    $21, $22, $23
)
RETURNING `+petColumns, append([]any{ownerID}, args...)...)
	return singlePet(row, "create pet")
}

func (s *Store) Update(ctx context.Context, ownerID, petID string, expectedVersion int64, details domain.PetDetails) (domain.Pet, error) {
	args, err := detailArgs(details)
	if err != nil {
		return domain.Pet{}, err
	}
	row := s.db.QueryRow(ctx, `UPDATE public.pets SET
    name = $4, species = $5, breed = $6, color = $7, sex = $8, birth_date = $9,
    chip_number = $10, weight_kg = $11, size = $12, distinctive_features = $13,
    allergies = $14, surgeries = $15, special_conditions = $16, temperament = $17,
    owner_full_name = $18, owner_address = $19, owner_comuna = $20, owner_phone = $21,
    owner_phone_alt = $22, second_owner_full_name = $23, second_owner_phone = $24,
    public_visibility = $25, version = version + 1
WHERE id = $1::uuid AND owner_id = $2::uuid AND version = $3
RETURNING `+petColumns, append([]any{petID, ownerID, expectedVersion}, args...)...)
	pet, err := singlePet(row, "update pet")
	if errors.Is(err, domain.ErrNotFound) {
		if _, getErr := s.GetForOwner(ctx, ownerID, petID); getErr == nil {
			return domain.Pet{}, domain.ErrStaleVersion
		}
	}
	return pet, err
}

func (s *Store) SetPhotoUpdatedAt(ctx context.Context, ownerID, petID string, at *time.Time) (domain.Pet, error) {
	row := s.db.QueryRow(ctx, `UPDATE public.pets SET photo_updated_at = $3
WHERE id = $1::uuid AND owner_id = $2::uuid
RETURNING `+petColumns, petID, ownerID, at)
	return singlePet(row, "set pet photo")
}

func (s *Store) Delete(ctx context.Context, ownerID, petID string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM public.pets WHERE id = $1::uuid AND owner_id = $2::uuid`, petID, ownerID)
	if err != nil {
		return fmt.Errorf("delete pet: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// PhotoStore keeps pet image bytes in PostgreSQL.
type PhotoStore struct {
	db *database.PostgresPgx
}

func NewPhotoStore(db *database.PostgresPgx) *PhotoStore {
	return &PhotoStore{db: db}
}

var _ ports.PetPhotoPort = (*PhotoStore)(nil)

func (s *PhotoStore) Get(ctx context.Context, petID string) (domain.PetPhoto, error) {
	var photo domain.PetPhoto
	err := s.db.QueryRow(ctx, `SELECT content_type, data, updated_at FROM public.pet_photos WHERE pet_id = $1::uuid`, petID).
		Scan(&photo.ContentType, &photo.Data, &photo.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PetPhoto{}, domain.ErrNotFound
		}
		return domain.PetPhoto{}, fmt.Errorf("get pet photo: %w", err)
	}
	return photo, nil
}

func (s *PhotoStore) Put(ctx context.Context, petID string, photo domain.PetPhoto) error {
	updatedAt := photo.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(ctx, `INSERT INTO public.pet_photos (pet_id, content_type, size_bytes, data, updated_at)
VALUES ($1::uuid, $2, $3, $4, $5)
ON CONFLICT (pet_id) DO UPDATE
SET content_type = EXCLUDED.content_type,
    size_bytes = EXCLUDED.size_bytes,
    data = EXCLUDED.data,
    updated_at = EXCLUDED.updated_at`, petID, photo.ContentType, len(photo.Data), photo.Data, updatedAt)
	if err != nil {
		return fmt.Errorf("put pet photo: %w", err)
	}
	return nil
}

func (s *PhotoStore) Delete(ctx context.Context, petID string) error {
	if _, err := s.db.Exec(ctx, `DELETE FROM public.pet_photos WHERE pet_id = $1::uuid`, petID); err != nil {
		return fmt.Errorf("delete pet photo: %w", err)
	}
	return nil
}

func detailArgs(d domain.PetDetails) ([]any, error) {
	visibility := make(map[string]string, len(d.PublicVisibility))
	for field, value := range d.PublicVisibility {
		visibility[string(field)] = string(value)
	}
	rawVisibility, err := json.Marshal(visibility)
	if err != nil {
		return nil, fmt.Errorf("encode pet visibility: %w", err)
	}
	var chip any
	if d.ChipNumber != "" {
		chip = d.ChipNumber
	}
	return []any{
		d.Name, string(d.Species), d.Breed, d.Color, string(d.Sex), d.BirthDate, chip, d.WeightKg, string(d.Size),
		d.DistinctiveFeatures, nonNil(d.Health.Allergies), nonNil(d.Health.Surgeries), nonNil(d.Health.SpecialConditions), d.Health.Temperament,
		d.Owner.FullName, d.Owner.Address, d.Owner.Comuna, d.Owner.Phone, d.Owner.PhoneAlt,
		d.SecondOwner.FullName, d.SecondOwner.Phone, rawVisibility,
	}, nil
}

func singlePet(row pgx.Row, op string) (domain.Pet, error) {
	pet, err := scanPet(row)
	if err == nil {
		return pet, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Pet{}, domain.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == chipUniqueIndex {
		return domain.Pet{}, domain.ErrDuplicateChip
	}
	return domain.Pet{}, fmt.Errorf("%s: %w", op, err)
}

func scanPet(row pgx.Row) (domain.Pet, error) {
	var (
		pet            domain.Pet
		species, sex   string
		size           string
		chip           *string
		rawVisibility  []byte
		photoUpdatedAt *time.Time
	)
	err := row.Scan(
		&pet.ID, &pet.OwnerID, &pet.Name, &species, &pet.Breed, &pet.Color, &sex, &pet.BirthDate, &chip,
		&pet.WeightKg, &size, &pet.DistinctiveFeatures, &pet.Health.Allergies, &pet.Health.Surgeries, &pet.Health.SpecialConditions,
		&pet.Health.Temperament, &pet.Owner.FullName, &pet.Owner.Address, &pet.Owner.Comuna, &pet.Owner.Phone, &pet.Owner.PhoneAlt,
		&pet.SecondOwner.FullName, &pet.SecondOwner.Phone, &rawVisibility, &photoUpdatedAt,
		&pet.Version, &pet.CreatedAt, &pet.UpdatedAt,
	)
	if err != nil {
		return domain.Pet{}, err
	}
	pet.Species = domain.PetSpecies(species)
	pet.Sex = domain.PetSex(sex)
	pet.Size = domain.PetSize(size)
	if chip != nil {
		pet.ChipNumber = *chip
	}
	stored := map[string]string{}
	if len(rawVisibility) > 0 {
		if err := json.Unmarshal(rawVisibility, &stored); err != nil {
			return domain.Pet{}, fmt.Errorf("decode pet visibility: %w", err)
		}
	}
	visibility := domain.DefaultPetVisibility()
	for field, value := range stored {
		if _, known := visibility[domain.PetPublicField(field)]; known {
			visibility[domain.PetPublicField(field)] = domain.PetVisibility(value)
		}
	}
	pet.PublicVisibility = visibility
	if photoUpdatedAt != nil {
		pet.HasPhoto = true
		pet.PhotoUpdatedAt = photoUpdatedAt.UTC()
	}
	pet.CreatedAt = pet.CreatedAt.UTC()
	pet.UpdatedAt = pet.UpdatedAt.UTC()
	return pet, nil
}

func nonNil(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}
