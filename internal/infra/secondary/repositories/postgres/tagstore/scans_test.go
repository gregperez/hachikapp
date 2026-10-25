package tagstore_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/database"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories/postgres/petstore"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories/postgres/tagstore"
	"github.com/jackc/pgx/v5/pgxpool"
)

type scanFixture struct {
	pool   *pgxpool.Pool
	store  *tagstore.Store
	pets   *petstore.Store
	owner  string
	petID  string
	tagID  string
	code   string
	prefix string
}

func newScanFixture(t *testing.T, key, codePrefix string) scanFixture {
	t.Helper()
	pool := testPool(t)
	db := database.NewPostgresPgxForTest(pool)
	store := tagstore.New(db)
	pets := petstore.New(db)
	ctx := context.Background()
	owner := createUser(t, pool, "scan-"+key+"@example.test")
	petID := createPet(t, pets, owner, "Luna")
	codes := newCodes(codePrefix, 1)
	if _, err := store.CreateBatch(ctx, domain.TagBatch{Label: "Scans", Quantity: 1, CreatedBy: owner}, codes); err != nil {
		t.Fatal(err)
	}
	tag, err := store.Activate(ctx, codes[0], owner, petID, domain.MaxActiveTagsPerPet, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	return scanFixture{pool: pool, store: store, pets: pets, owner: owner, petID: petID, tagID: tag.ID, code: codes[0], prefix: key}
}

func (f scanFixture) scan(visitor, token string, at time.Time) domain.TagScan {
	return domain.TagScan{
		TagID: f.tagID, PetID: f.petID, ScannedAt: at,
		VisitorHash:     domain.KeyedHash([]byte("p"), f.prefix+visitor),
		ScanTokenHash:   domain.KeyedHash([]byte("p"), f.prefix+token),
		IPPrefix:        "203.0.113.0/24",
		UserAgentFamily: "ios",
	}
}

func TestRecordScanDedupes(t *testing.T) {
	f := newScanFixture(t, "dedupe", "5CND0001")
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	first, reused, err := f.store.RecordScan(ctx, f.scan("v1", "t1", now), now.Add(-domain.ScanDedupeWindow), nil)
	if err != nil || reused || first.ID == "" {
		t.Fatalf("first=%+v reused=%v err=%v", first, reused, err)
	}
	if first.IPPrefix != "203.0.113.0/24" || first.UserAgentFamily != "ios" || !first.ScannedAt.Equal(now) {
		t.Fatalf("first=%+v", first)
	}

	later := now.Add(5 * time.Minute)
	second, reused, err := f.store.RecordScan(ctx, f.scan("v1", "t2", later), later.Add(-domain.ScanDedupeWindow), nil)
	if err != nil || !reused || second.ID != first.ID {
		t.Fatalf("second=%+v reused=%v err=%v", second, reused, err)
	}
	if !second.ScannedAt.Equal(now) || second.ScanTokenHash != domain.KeyedHash([]byte("p"), "dedupet2") {
		t.Fatalf("second=%+v", second)
	}
	got, err := f.store.GetScan(ctx, first.ID)
	if err != nil || got.ScanTokenHash != second.ScanTokenHash {
		t.Fatalf("got=%+v err=%v", got, err)
	}

	// A different visitor is a different scan.
	other, reused, err := f.store.RecordScan(ctx, f.scan("v2", "t3", later), later.Add(-domain.ScanDedupeWindow), nil)
	if err != nil || reused || other.ID == first.ID {
		t.Fatalf("other=%+v reused=%v err=%v", other, reused, err)
	}

	// Outside the window a new scan is stored.
	far := now.Add(30 * time.Minute)
	third, reused, err := f.store.RecordScan(ctx, f.scan("v1", "t4", far), far.Add(-domain.ScanDedupeWindow), nil)
	if err != nil || reused || third.ID == first.ID {
		t.Fatalf("third=%+v reused=%v err=%v", third, reused, err)
	}
}

func TestRecordScanConcurrentSameVisitorInsertsOnce(t *testing.T) {
	f := newScanFixture(t, "conc", "5CNC0002")
	ctx := context.Background()
	now := time.Now().UTC()
	const n = 8
	type result struct {
		id     string
		reused bool
		err    error
	}
	results := make(chan result, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			s, reused, err := f.store.RecordScan(ctx, f.scan("same", "tok"+string(rune('a'+i)), now), now.Add(-domain.ScanDedupeWindow), nil)
			results <- result{s.ID, reused, err}
		}(i)
	}
	ids := map[string]bool{}
	fresh := 0
	for i := 0; i < n; i++ {
		r := <-results
		if r.err != nil {
			t.Fatal(r.err)
		}
		ids[r.id] = true
		if !r.reused {
			fresh++
		}
	}
	if len(ids) != 1 || fresh != 1 {
		t.Fatalf("ids=%d fresh=%d", len(ids), fresh)
	}
}

func TestGetScanUnknown(t *testing.T) {
	f := newScanFixture(t, "getunk", "5CNG0003")
	for _, id := range []string{"00000000-0000-0000-0000-000000000000", "not-a-uuid"} {
		if _, err := f.store.GetScan(context.Background(), id); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("id=%q err=%v", id, err)
		}
	}
}

func TestMarkPhoneRevealedOnce(t *testing.T) {
	f := newScanFixture(t, "reveal", "5CNR0004")
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	scan, _, err := f.store.RecordScan(ctx, f.scan("v1", "t1", now), now.Add(-domain.ScanDedupeWindow), nil)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := f.store.MarkPhoneRevealed(ctx, scan.ID, now, nil)
	if err != nil || !ok {
		t.Fatalf("first ok=%v err=%v", ok, err)
	}
	ok, err = f.store.MarkPhoneRevealed(ctx, scan.ID, now.Add(time.Minute), nil)
	if err != nil || ok {
		t.Fatalf("second ok=%v err=%v", ok, err)
	}
	got, err := f.store.GetScan(ctx, scan.ID)
	if err != nil || got.PhoneRevealedAt == nil || !got.PhoneRevealedAt.Equal(now) {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if _, err := f.store.MarkPhoneRevealed(ctx, "00000000-0000-0000-0000-000000000000", now, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown err=%v", err)
	}
}

func TestListScansByPetIsOwnerScoped(t *testing.T) {
	f := newScanFixture(t, "list", "5CNM0005")
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Hour)

	var scans []domain.TagScan
	for i := 0; i < 3; i++ {
		at := base.Add(time.Duration(i) * time.Minute)
		s, _, err := f.store.RecordScan(ctx, f.scan("v"+string(rune('a'+i)), "t"+string(rune('a'+i)), at), at.Add(-domain.ScanDedupeWindow), nil)
		if err != nil {
			t.Fatal(err)
		}
		scans = append(scans, s)
	}
	for i, body := range []string{"primero", "segundo"} {
		msg, err := f.store.AddMessage(ctx, domain.TagMessage{
			ScanID: scans[1].ID, TagID: f.tagID, PetID: f.petID, Body: body, FinderName: "Ana", FinderPhone: "+56 9 1234 5678",
		}, nil)
		if err != nil || msg.ID == "" || msg.CreatedAt.IsZero() || msg.Body != body || msg.FinderName != "Ana" {
			t.Fatalf("msg %d=%+v err=%v", i, msg, err)
		}
	}

	page, err := f.store.ListScansByPet(ctx, f.owner, f.petID, authBaseDomain.PageRequest{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || !page.HasNext || page.Items[0].ID != scans[2].ID || page.Items[1].ID != scans[1].ID {
		t.Fatalf("page=%+v", page)
	}
	if page.Items[0].TagCode != f.code || len(page.Items[0].Messages) != 0 {
		t.Fatalf("item0=%+v", page.Items[0])
	}
	msgs := page.Items[1].Messages
	if len(msgs) != 2 || msgs[0].Body != "primero" || msgs[1].Body != "segundo" {
		t.Fatalf("msgs=%+v", msgs)
	}

	last := page.Items[1]
	next, err := f.store.ListScansByPet(ctx, f.owner, f.petID, authBaseDomain.PageRequest{
		Limit: 2, After: &authBaseDomain.CursorKey{CreatedAt: last.ScannedAt, ID: last.ID},
	})
	if err != nil || len(next.Items) != 1 || next.HasNext || next.Items[0].ID != scans[0].ID {
		t.Fatalf("next=%+v err=%v", next, err)
	}

	ownerB := createUser(t, f.pool, "scan-list-b@example.test")
	other, err := f.store.ListScansByPet(ctx, ownerB, f.petID, authBaseDomain.PageRequest{Limit: 10})
	if err != nil || len(other.Items) != 0 || other.HasNext {
		t.Fatalf("other=%+v err=%v", other, err)
	}
}

func TestPetDeletionRemovesScansAndMessages(t *testing.T) {
	f := newScanFixture(t, "cascade", "5CNK0006")
	ctx := context.Background()
	now := time.Now().UTC()
	scan, _, err := f.store.RecordScan(ctx, f.scan("v1", "t1", now), now.Add(-domain.ScanDedupeWindow), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.AddMessage(ctx, domain.TagMessage{ScanID: scan.ID, TagID: f.tagID, PetID: f.petID, Body: strings.Repeat("a", 10)}, nil); err != nil {
		t.Fatal(err)
	}
	if err := f.pets.Delete(ctx, f.owner, f.petID); err != nil {
		t.Fatal(err)
	}
	var scans, messages int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM public.tag_scans WHERE pet_id = $1::uuid`, f.petID).Scan(&scans); err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM public.tag_messages WHERE pet_id = $1::uuid`, f.petID).Scan(&messages); err != nil {
		t.Fatal(err)
	}
	if scans != 0 || messages != 0 {
		t.Fatalf("scans=%d messages=%d", scans, messages)
	}
}

func (f scanFixture) draft(kind domain.NotificationKind, userID string) *domain.NotificationDraft {
	return &domain.NotificationDraft{
		UserID: userID, PetID: f.petID, TagID: f.tagID, Kind: kind, Email: true,
		Payload: domain.NotificationPayload{PetName: "Luna", TagCode: f.code, ScannedAt: time.Now().UTC(), Device: "ios"},
	}
}

func (f scanFixture) counts(t *testing.T) (notifications, deliveries int) {
	t.Helper()
	ctx := context.Background()
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM public.notifications WHERE tag_id = $1::uuid`, f.tagID).Scan(&notifications); err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM public.notification_deliveries d
JOIN public.notifications n ON n.id = d.notification_id WHERE n.tag_id = $1::uuid`, f.tagID).Scan(&deliveries); err != nil {
		t.Fatal(err)
	}
	return notifications, deliveries
}

func TestRecordScanNewCreatesOneNotificationAndDelivery(t *testing.T) {
	f := newScanFixture(t, "notify1", "5CNN0010")
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, reused, err := f.store.RecordScan(ctx, f.scan("v1", "t1", now), now.Add(-domain.ScanDedupeWindow), f.draft(domain.NotificationTagScanned, f.owner)); err != nil || reused {
		t.Fatalf("reused=%v err=%v", reused, err)
	}
	if n, d := f.counts(t); n != 1 || d != 1 {
		t.Fatalf("notifications=%d deliveries=%d", n, d)
	}
}

func TestRecordScanReuseCreatesNoSecondNotification(t *testing.T) {
	f := newScanFixture(t, "notify2", "5CNN0011")
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	draft := f.draft(domain.NotificationTagScanned, f.owner)
	if _, _, err := f.store.RecordScan(ctx, f.scan("v1", "t1", now), now.Add(-domain.ScanDedupeWindow), draft); err != nil {
		t.Fatal(err)
	}
	later := now.Add(time.Minute)
	if _, reused, err := f.store.RecordScan(ctx, f.scan("v1", "t2", later), later.Add(-domain.ScanDedupeWindow), draft); err != nil || !reused {
		t.Fatalf("reused=%v err=%v", reused, err)
	}
	if n, d := f.counts(t); n != 1 || d != 1 {
		t.Fatalf("notifications=%d deliveries=%d", n, d)
	}
}

func TestAddMessageWithinMinuteGroupsDelivery(t *testing.T) {
	f := newScanFixture(t, "notify3", "5CNN0012")
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	scan, _, err := f.store.RecordScan(ctx, f.scan("v1", "t1", now), now.Add(-domain.ScanDedupeWindow), f.draft(domain.NotificationTagScanned, f.owner))
	if err != nil {
		t.Fatal(err)
	}
	msg := domain.TagMessage{ScanID: scan.ID, TagID: f.tagID, PetID: f.petID, Body: strings.Repeat("a", 10), CreatedAt: now.Add(10 * time.Second)}
	msgDraft := f.draft(domain.NotificationFinderMessage, f.owner)
	msgDraft.Payload.Message = &domain.TagMessageSummary{Body: msg.Body}
	stored, err := f.store.AddMessage(ctx, msg, msgDraft)
	if err != nil || !stored.CreatedAt.Equal(msg.CreatedAt) {
		t.Fatalf("stored=%+v err=%v", stored, err)
	}
	if n, d := f.counts(t); n != 2 || d != 1 {
		t.Fatalf("notifications=%d deliveries=%d", n, d)
	}
}

func TestMarkPhoneRevealedNotifiesOnlyFirstTime(t *testing.T) {
	f := newScanFixture(t, "notify4", "5CNN0013")
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	scan, _, err := f.store.RecordScan(ctx, f.scan("v1", "t1", now), now.Add(-domain.ScanDedupeWindow), nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := f.store.MarkPhoneRevealed(ctx, scan.ID, now.Add(time.Duration(i)*time.Second), f.draft(domain.NotificationPhoneRevealed, f.owner)); err != nil {
			t.Fatal(err)
		}
	}
	if n, _ := f.counts(t); n != 1 {
		t.Fatalf("notifications=%d", n)
	}
}

func TestEventRollsBackWithNotification(t *testing.T) {
	f := newScanFixture(t, "rollback", "5CNR0014")
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	bad := f.draft(domain.NotificationTagScanned, "00000000-0000-0000-0000-000000000000")

	if _, _, err := f.store.RecordScan(ctx, f.scan("v1", "t1", now), now.Add(-domain.ScanDedupeWindow), bad); err == nil {
		t.Fatal("want error")
	}
	var scans int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM public.tag_scans WHERE tag_id = $1::uuid`, f.tagID).Scan(&scans); err != nil || scans != 0 {
		t.Fatalf("scans=%d err=%v", scans, err)
	}

	scan, _, err := f.store.RecordScan(ctx, f.scan("v1", "t1", now), now.Add(-domain.ScanDedupeWindow), nil)
	if err != nil {
		t.Fatal(err)
	}
	bad.Kind = domain.NotificationFinderMessage
	if _, err := f.store.AddMessage(ctx, domain.TagMessage{ScanID: scan.ID, TagID: f.tagID, PetID: f.petID, Body: strings.Repeat("a", 10)}, bad); err == nil {
		t.Fatal("want error")
	}
	var messages int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM public.tag_messages WHERE tag_id = $1::uuid`, f.tagID).Scan(&messages); err != nil || messages != 0 {
		t.Fatalf("messages=%d err=%v", messages, err)
	}

	bad.Kind = domain.NotificationPhoneRevealed
	if _, err := f.store.MarkPhoneRevealed(ctx, scan.ID, now, bad); err == nil {
		t.Fatal("want error")
	}
	got, err := f.store.GetScan(ctx, scan.ID)
	if err != nil || got.PhoneRevealedAt != nil {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}
