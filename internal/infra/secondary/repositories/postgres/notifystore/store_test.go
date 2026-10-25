package notifystore_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/database"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories/postgres/notifystore"
	"github.com/gregperez/hachikapp-backend/internal/infra/secondary/repositories/postgres/petstore"
	"github.com/gregperez/hachikapp-backend/internal/test/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests expect a migrated DATABASE_TEST_URL and skip otherwise.
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
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.notification_deliveries') IS NOT NULL`).Scan(&exists); err != nil {
		t.Fatalf("check schema: %v", err)
	}
	if !exists {
		t.Skip("public.notification_deliveries missing: run migrations first")
	}
	return pool
}

type fixture struct {
	pool   *pgxpool.Pool
	store  *notifystore.Store
	userID string
	petID  string
	tagID  string
	email  string
}

var codeSeq int

func newFixture(t *testing.T, email string) fixture {
	t.Helper()
	pool := testPool(t)
	ctx := context.Background()
	db := database.NewPostgresPgxForTest(pool)
	_, _ = pool.Exec(ctx, `DELETE FROM public.users WHERE email = $1`, email)
	var userID string
	if err := pool.QueryRow(ctx, `INSERT INTO public.users (name, email, password, active)
VALUES ('Ana Perez Soto', $1, 'x', true) RETURNING id::text`, email).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM public.pet_tags WHERE owner_id = $1::uuid`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM public.users WHERE id = $1::uuid`, userID)
	})
	pet, err := petstore.New(db).Create(ctx, userID, domain.NormalizePetDetails(domain.PetDetails{
		Name: "Luna", Species: domain.PetSpeciesDog,
		Owner: domain.PetOwnerContact{FullName: "Ana", Phone: "+56 9 1234 5678"},
	}))
	if err != nil {
		t.Fatalf("create pet: %v", err)
	}
	codeSeq++
	code := fmt.Sprintf("NT%08d", (time.Now().UnixNano()/1000+int64(codeSeq))%100000000)
	var tagID string
	if err := pool.QueryRow(ctx, `INSERT INTO public.pet_tags (code, kind, status, pet_id, owner_id, activated_at)
VALUES ($1, 'digital', 'active', $2::uuid, $3::uuid, now()) RETURNING id::text`, code, pet.ID, userID).Scan(&tagID); err != nil {
		t.Fatalf("insert tag: %v", err)
	}
	return fixture{pool: pool, store: notifystore.New(db), userID: userID, petID: pet.ID, tagID: tagID, email: email}
}

func (f fixture) newScan(t *testing.T) string {
	t.Helper()
	codeSeq++
	h := fmt.Sprintf("%064d", time.Now().UnixNano()+int64(codeSeq))
	var id string
	if err := f.pool.QueryRow(context.Background(), `INSERT INTO public.tag_scans (tag_id, pet_id, visitor_hash, scan_token_hash)
VALUES ($1::uuid, $2::uuid, $3, $3) RETURNING id::text`, f.tagID, f.petID, h).Scan(&id); err != nil {
		t.Fatalf("insert scan: %v", err)
	}
	return id
}

func (f fixture) newMessage(t *testing.T, scanID string) string {
	t.Helper()
	var id string
	if err := f.pool.QueryRow(context.Background(), `INSERT INTO public.tag_messages (scan_id, tag_id, pet_id, body)
VALUES ($1::uuid, $2::uuid, $3::uuid, 'hola') RETURNING id::text`, scanID, f.tagID, f.petID).Scan(&id); err != nil {
		t.Fatalf("insert message: %v", err)
	}
	return id
}

func (f fixture) draft(kind domain.NotificationKind, email bool) domain.NotificationDraft {
	return domain.NotificationDraft{
		UserID: f.userID, PetID: f.petID, TagID: f.tagID, Kind: kind, Email: email,
		Payload: domain.NotificationPayload{PetName: "Luna", TagCode: "ABC", ScannedAt: time.Now().UTC().Truncate(time.Second)},
	}
}

func (f fixture) insert(t *testing.T, d domain.NotificationDraft, scanID, messageID string, now time.Time) {
	t.Helper()
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := notifystore.InsertEvent(ctx, tx, d, scanID, messageID, now); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("InsertEvent: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func (f fixture) deliveryCount(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM public.notification_deliveries d
JOIN public.notifications n ON n.id = d.notification_id WHERE n.user_id = $1::uuid`, f.userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (f fixture) notifications(t *testing.T) []domain.Notification {
	t.Helper()
	page, err := f.store.List(context.Background(), f.userID, authBaseDomain.PageRequest{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	return page.Items
}

func TestInsertEventScanCreatesPendingEmail(t *testing.T) {
	f := newFixture(t, "notif-scan@example.test")
	now := time.Now().UTC().Truncate(time.Millisecond)
	scan := f.newScan(t)
	f.insert(t, f.draft(domain.NotificationTagScanned, true), scan, "", now)

	items := f.notifications(t)
	if len(items) != 1 || items[0].Kind != domain.NotificationTagScanned || items[0].ScanID != scan || items[0].Payload.PetName != "Luna" {
		t.Fatalf("items=%+v", items)
	}
	var status string
	var attempts int
	var next time.Time
	if err := f.pool.QueryRow(context.Background(), `SELECT status, attempts, next_attempt_at FROM public.notification_deliveries WHERE notification_id = $1::uuid`, items[0].ID).Scan(&status, &attempts, &next); err != nil {
		t.Fatal(err)
	}
	if status != "pending" || attempts != 0 || !next.Equal(now.Add(domain.FirstEmailDelay)) {
		t.Fatalf("status=%s attempts=%d next=%v", status, attempts, next)
	}
}

func TestInsertEventGroupsIntoPendingScanEmail(t *testing.T) {
	f := newFixture(t, "notif-group@example.test")
	now := time.Now().UTC().Truncate(time.Millisecond)
	scan := f.newScan(t)
	msg := f.newMessage(t, scan)
	f.insert(t, f.draft(domain.NotificationTagScanned, true), scan, "", now)

	d := f.draft(domain.NotificationFinderMessage, true)
	d.Payload.Message = &domain.TagMessageSummary{Body: "la encontré", FinderName: "Pedro", FinderPhone: "+56911111111"}
	f.insert(t, d, scan, msg, now.Add(10*time.Second))
	loc := f.draft(domain.NotificationLocationShared, true)
	loc.Payload.Location = &domain.ScanLocation{Lat: -33.4, Lng: -70.6, AccuracyM: 20, At: now}
	f.insert(t, loc, scan, "", now.Add(11*time.Second))
	ph := f.draft(domain.NotificationPhoneRevealed, true)
	f.insert(t, ph, scan, "", now.Add(12*time.Second))

	if n := f.deliveryCount(t); n != 1 {
		t.Fatalf("deliveries=%d want 1", n)
	}
	items := f.notifications(t)
	if len(items) != 4 {
		t.Fatalf("notifications=%d want 4", len(items))
	}
	for _, it := range items {
		if it.Kind == domain.NotificationFinderMessage && it.MessageID != msg {
			t.Fatalf("message id=%q", it.MessageID)
		}
		if it.Kind != domain.NotificationTagScanned {
			continue
		}
		p := it.Payload
		if p.Message == nil || p.Message.Body != "la encontré" || p.Message.FinderName != "Pedro" ||
			p.Location == nil || p.Location.Lat != -33.4 || !p.PhoneRevealed || p.PetName != "Luna" {
			t.Fatalf("merged payload=%+v", p)
		}
	}
}

func TestInsertEventSecondFinderMessageGetsOwnEmail(t *testing.T) {
	f := newFixture(t, "notif-two-msgs@example.test")
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	scan := f.newScan(t)
	f.insert(t, f.draft(domain.NotificationTagScanned, true), scan, "", now)

	first := f.draft(domain.NotificationFinderMessage, true)
	first.Payload.Message = &domain.TagMessageSummary{Body: "primer mensaje", FinderName: "Pedro"}
	f.insert(t, first, scan, f.newMessage(t, scan), now.Add(10*time.Second))
	second := f.draft(domain.NotificationFinderMessage, true)
	second.Payload.Message = &domain.TagMessageSummary{Body: "segundo mensaje", FinderName: "Pedro"}
	secondAt := now.Add(20 * time.Second)
	f.insert(t, second, scan, f.newMessage(t, scan), secondAt)

	if n := f.deliveryCount(t); n != 2 {
		t.Fatalf("deliveries=%d want 2 (scan email + second message)", n)
	}
	var kinds []string
	rows, err := f.pool.Query(ctx, `SELECT n.kind, n.payload->'message'->>'body', d.next_attempt_at
FROM public.notification_deliveries d JOIN public.notifications n ON n.id = d.notification_id
WHERE n.user_id = $1::uuid ORDER BY n.created_at`, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var body *string
		var next time.Time
		if err := rows.Scan(&kind, &body, &next); err != nil {
			t.Fatal(err)
		}
		kinds = append(kinds, kind)
		switch kind {
		case string(domain.NotificationTagScanned):
			if body == nil || *body != "primer mensaje" {
				t.Fatalf("scan email message=%v want first message", body)
			}
			if !next.Equal(now.Add(domain.FirstEmailDelay)) {
				t.Fatalf("scan next=%v", next)
			}
		case string(domain.NotificationFinderMessage):
			if body == nil || *body != "segundo mensaje" {
				t.Fatalf("own delivery message=%v want second message", body)
			}
			if !next.Equal(secondAt) {
				t.Fatalf("second next=%v want %v", next, secondAt)
			}
		default:
			t.Fatalf("unexpected delivery kind %s", kind)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(kinds) != 2 || kinds[0] == kinds[1] {
		t.Fatalf("delivery kinds=%v", kinds)
	}
}

func TestInsertEventNoGroupingAfterDue(t *testing.T) {
	f := newFixture(t, "notif-due@example.test")
	now := time.Now().UTC().Truncate(time.Millisecond)
	scan := f.newScan(t)
	f.insert(t, f.draft(domain.NotificationTagScanned, true), scan, "", now)

	// Past the first-email delay the scan delivery is due, so the event sends on its own.
	later := now.Add(domain.FirstEmailDelay + time.Second)
	f.insert(t, f.draft(domain.NotificationFinderMessage, true), scan, "", later)
	if n := f.deliveryCount(t); n != 2 {
		t.Fatalf("deliveries after due=%d want 2", n)
	}
	var next time.Time
	if err := f.pool.QueryRow(context.Background(), `SELECT d.next_attempt_at FROM public.notification_deliveries d
JOIN public.notifications n ON n.id = d.notification_id WHERE n.kind = 'finder_message' AND n.user_id = $1::uuid`, f.userID).Scan(&next); err != nil {
		t.Fatal(err)
	}
	if !next.Equal(later) {
		t.Fatalf("next=%v want %v", next, later)
	}

	// A leased delivery (claimed, attempts still 0, next_attempt_at pushed past the
	// lease) is still in the future, so only the created_at+FirstEmailDelay guard
	// keeps a new event from merging into an email that is already being sent.
	scan2 := f.newScan(t)
	f.insert(t, f.draft(domain.NotificationTagScanned, true), scan2, "", now)
	claimed, err := f.store.ClaimDue(context.Background(), now.Add(domain.FirstEmailDelay), 100, domain.DeliveryLease)
	if err != nil {
		t.Fatal(err)
	}
	leased := false
	for _, c := range claimed {
		leased = leased || c.Notification.ScanID == scan2
	}
	if !leased {
		t.Fatal("scan2 delivery was not claimed")
	}
	msg2 := f.draft(domain.NotificationFinderMessage, true)
	msg2.Payload.Message = &domain.TagMessageSummary{Body: "hola"}
	f.insert(t, msg2, scan2, "", now.Add(domain.FirstEmailDelay+10*time.Second))
	if n := f.deliveryCount(t); n != 4 {
		t.Fatalf("deliveries=%d want 4 (finder_message must get its own)", n)
	}
	for _, it := range f.notifications(t) {
		if it.ScanID == scan2 && it.Kind == domain.NotificationTagScanned && it.Payload.Message != nil {
			t.Fatalf("leased scan payload was merged: %+v", it.Payload)
		}
	}

	// Attempts > 0 also blocks grouping.
	scan3 := f.newScan(t)
	f.insert(t, f.draft(domain.NotificationTagScanned, true), scan3, "", now)
	if _, err := f.pool.Exec(context.Background(), `UPDATE public.notification_deliveries d SET attempts = 1
FROM public.notifications n WHERE n.id = d.notification_id AND n.scan_id = $1::uuid`, scan3); err != nil {
		t.Fatal(err)
	}
	f.insert(t, f.draft(domain.NotificationFinderMessage, true), scan3, "", now.Add(time.Second))
	if n := f.deliveryCount(t); n != 6 {
		t.Fatalf("deliveries=%d want 6", n)
	}
}

func TestInsertEventWithoutEmail(t *testing.T) {
	f := newFixture(t, "notif-noemail@example.test")
	now := time.Now().UTC()
	scan := f.newScan(t)
	f.insert(t, f.draft(domain.NotificationTagScanned, false), scan, "", now)
	f.insert(t, f.draft(domain.NotificationFinderMessage, false), scan, "", now)
	if n := f.deliveryCount(t); n != 0 {
		t.Fatalf("deliveries=%d", n)
	}
	if len(f.notifications(t)) != 2 {
		t.Fatal("expected 2 notifications")
	}
	// An Email=false event never groups into an email-bearing scan.
	emailScan := f.newScan(t)
	f.insert(t, f.draft(domain.NotificationTagScanned, true), emailScan, "", now)
	before := f.deliveryCount(t)
	quiet := f.draft(domain.NotificationFinderMessage, false)
	quiet.Payload.Message = &domain.TagMessageSummary{Body: "hola"}
	f.insert(t, quiet, emailScan, "", now.Add(time.Second))
	if n := f.deliveryCount(t); n != before {
		t.Fatalf("deliveries=%d want %d", n, before)
	}
	for _, it := range f.notifications(t) {
		if it.ScanID == emailScan && it.Kind == domain.NotificationTagScanned && it.Payload.Message != nil {
			t.Fatalf("scan payload changed: %+v", it.Payload)
		}
	}
}

func (f fixture) seedDue(t *testing.T, n int, now time.Time) {
	t.Helper()
	for i := 0; i < n; i++ {
		f.insert(t, f.draft(domain.NotificationTagScanned, true), f.newScan(t), "", now.Add(-domain.FirstEmailDelay-time.Minute-time.Duration(i)*time.Millisecond))
	}
}

func TestClaimDueSkipLocked(t *testing.T) {
	f := newFixture(t, "notif-claim@example.test")
	now := time.Now().UTC()
	f.seedDue(t, 30, now)

	var wg sync.WaitGroup
	results := make([][]domain.Delivery, 2)
	errs := make([]error, 2)
	start := make(chan struct{})
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = f.store.ClaimDue(context.Background(), now, 20, domain.DeliveryLease)
		}(i)
	}
	close(start)
	wg.Wait()
	seen := map[string]bool{}
	total := 0
	for i, r := range results {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		for _, d := range r {
			if seen[d.ID] {
				t.Fatalf("delivery %s claimed twice", d.ID)
			}
			seen[d.ID] = true
			total++
		}
	}
	if total != 30 {
		t.Fatalf("claimed %d want 30 (sizes %d/%d)", total, len(results[0]), len(results[1]))
	}
}

func TestClaimDueReclaimsExpiredLease(t *testing.T) {
	f := newFixture(t, "notif-lease@example.test")
	now := time.Now().UTC()
	f.seedDue(t, 1, now)
	first, err := f.store.ClaimDue(context.Background(), now, 10, domain.DeliveryLease)
	if err != nil || len(first) != 1 {
		t.Fatalf("first=%v err=%v", first, err)
	}
	again, err := f.store.ClaimDue(context.Background(), now.Add(time.Minute), 10, domain.DeliveryLease)
	if err != nil || len(again) != 0 {
		t.Fatalf("during lease=%v err=%v", again, err)
	}
	later, err := f.store.ClaimDue(context.Background(), now.Add(domain.DeliveryLease+time.Second), 10, domain.DeliveryLease)
	if err != nil || len(later) != 1 || later[0].ID != first[0].ID {
		t.Fatalf("after lease=%v err=%v", later, err)
	}
}

func TestClaimDueJoinsOwnerEmail(t *testing.T) {
	f := newFixture(t, "notif-join@example.test")
	now := time.Now().UTC()
	f.seedDue(t, 1, now)
	got, err := f.store.ClaimDue(context.Background(), now, 10, domain.DeliveryLease)
	if err != nil || len(got) != 1 {
		t.Fatalf("got=%v err=%v", got, err)
	}
	d := got[0]
	if d.ToEmail != "notif-join@example.test" || d.OwnerName != "Ana Perez Soto" || d.Attempts != 0 ||
		d.NotificationID != d.Notification.ID || d.Notification.Kind != domain.NotificationTagScanned ||
		d.Notification.Payload.PetName != "Luna" || d.Notification.UserID != f.userID {
		t.Fatalf("delivery=%+v", d)
	}
}

func TestMarkSentRetryAndFailed(t *testing.T) {
	f := newFixture(t, "notif-mark@example.test")
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	f.seedDue(t, 3, now)
	got, err := f.store.ClaimDue(ctx, now, 10, domain.DeliveryLease)
	if err != nil || len(got) != 3 {
		t.Fatalf("got=%v err=%v", got, err)
	}
	read := func(id string) (status string, attempts int, next time.Time, lastErr string, sent *time.Time) {
		if err := f.pool.QueryRow(ctx, `SELECT status, attempts, next_attempt_at, last_error, sent_at FROM public.notification_deliveries WHERE id = $1::uuid`, id).
			Scan(&status, &attempts, &next, &lastErr, &sent); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err := f.store.MarkSent(ctx, got[0].ID, now); err != nil {
		t.Fatal(err)
	}
	if st, _, _, _, sent := read(got[0].ID); st != "sent" || sent == nil || !sent.Equal(now) {
		t.Fatalf("sent status=%s at=%v", st, sent)
	}
	retryAt := now.Add(time.Minute)
	long := strings.Repeat("ñ", 500)
	if err := f.store.MarkRetry(ctx, got[1].ID, 1, retryAt, long); err != nil {
		t.Fatal(err)
	}
	st, att, next, le, _ := read(got[1].ID)
	if st != "pending" || att != 1 || !next.Equal(retryAt) || len([]rune(le)) != 300 {
		t.Fatalf("retry st=%s att=%d next=%v len=%d", st, att, next, len([]rune(le)))
	}
	if err := f.store.MarkFailed(ctx, got[2].ID, 6, "boom"); err != nil {
		t.Fatal(err)
	}
	if st, att, _, le, _ := read(got[2].ID); st != "failed" || att != 6 || le != "boom" {
		t.Fatalf("failed st=%s att=%d le=%s", st, att, le)
	}
	// Late marks on a delivery another worker already finished are no-ops.
	if err := f.store.MarkRetry(ctx, got[0].ID, 2, retryAt, "late"); err != nil {
		t.Fatalf("late retry on sent: %v", err)
	}
	if err := f.store.MarkFailed(ctx, got[0].ID, 6, "late"); err != nil {
		t.Fatalf("late failed on sent: %v", err)
	}
	if st, att, _, le, sent := read(got[0].ID); st != "sent" || att != 0 || le != "" || sent == nil || !sent.Equal(now) {
		t.Fatalf("sent delivery overwritten: st=%s att=%d le=%q sent=%v", st, att, le, sent)
	}
	if err := f.store.MarkSent(ctx, got[2].ID, now.Add(time.Hour)); err != nil {
		t.Fatalf("late sent on failed: %v", err)
	}
	if err := f.store.MarkRetry(ctx, got[2].ID, 7, retryAt, "late"); err != nil {
		t.Fatalf("late retry on failed: %v", err)
	}
	if st, att, _, le, sent := read(got[2].ID); st != "failed" || att != 6 || le != "boom" || sent != nil {
		t.Fatalf("failed delivery overwritten: st=%s att=%d le=%q sent=%v", st, att, le, sent)
	}
	missing := "00000000-0000-0000-0000-000000000000"
	if err := f.store.MarkSent(ctx, missing, now); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("sent unknown: %v", err)
	}
	if err := f.store.MarkRetry(ctx, missing, 1, now, "x"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("retry unknown: %v", err)
	}
	if err := f.store.MarkFailed(ctx, missing, 1, "x"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("failed unknown: %v", err)
	}
}

func TestListAndUnreadAreOwnerScoped(t *testing.T) {
	a := newFixture(t, "notif-list-a@example.test")
	b := newFixture(t, "notif-list-b@example.test")
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Millisecond)
	for i := 0; i < 3; i++ {
		a.insert(t, a.draft(domain.NotificationTagScanned, false), a.newScan(t), "", base.Add(time.Duration(i)*time.Second))
	}
	if n, err := a.store.UnreadCount(ctx, a.userID); err != nil || n != 3 {
		t.Fatalf("unread a=%d err=%v", n, err)
	}
	if n, err := a.store.UnreadCount(ctx, b.userID); err != nil || n != 0 {
		t.Fatalf("unread b=%d err=%v", n, err)
	}
	if items := b.notifications(t); len(items) != 0 {
		t.Fatalf("b sees %d", len(items))
	}
	first, err := a.store.List(ctx, a.userID, authBaseDomain.PageRequest{Limit: 2})
	if err != nil || len(first.Items) != 2 || !first.HasNext {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	next, err := a.store.List(ctx, a.userID, authBaseDomain.PageRequest{Limit: 2, After: &authBaseDomain.CursorKey{CreatedAt: first.Items[1].CreatedAt, ID: first.Items[1].ID}})
	if err != nil || len(next.Items) != 1 || next.HasNext {
		t.Fatalf("next=%+v err=%v", next, err)
	}
	ids := []string{first.Items[0].ID, first.Items[1].ID, next.Items[0].ID}
	sort.Strings(ids)
	if ids[0] == ids[1] || ids[1] == ids[2] {
		t.Fatalf("duplicate ids %v", ids)
	}
	if !first.Items[0].CreatedAt.After(next.Items[0].CreatedAt) && !first.Items[0].CreatedAt.Equal(next.Items[0].CreatedAt) {
		t.Fatal("not newest first")
	}
}

func TestMarkReadForeignIsNotFound(t *testing.T) {
	a := newFixture(t, "notif-read-a@example.test")
	b := newFixture(t, "notif-read-b@example.test")
	ctx := context.Background()
	now := time.Now().UTC()
	a.insert(t, a.draft(domain.NotificationTagScanned, false), a.newScan(t), "", now)
	id := a.notifications(t)[0].ID

	if err := b.store.MarkRead(ctx, b.userID, id, now); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign: %v", err)
	}
	if err := a.store.MarkRead(ctx, a.userID, "not-a-uuid", now); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("malformed: %v", err)
	}
	if err := a.store.MarkRead(ctx, a.userID, "00000000-0000-0000-0000-000000000000", now); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
	if err := a.store.MarkRead(ctx, a.userID, id, now); err != nil {
		t.Fatal(err)
	}
	if err := a.store.MarkRead(ctx, a.userID, id, now.Add(time.Hour)); err != nil {
		t.Fatalf("idempotent: %v", err)
	}
	got := a.notifications(t)[0]
	if got.ReadAt == nil || !got.ReadAt.Equal(now.Truncate(time.Microsecond)) {
		t.Fatalf("readAt=%v want first read time", got.ReadAt)
	}
}

func TestMarkAllRead(t *testing.T) {
	a := newFixture(t, "notif-all-a@example.test")
	b := newFixture(t, "notif-all-b@example.test")
	ctx := context.Background()
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		a.insert(t, a.draft(domain.NotificationTagScanned, false), a.newScan(t), "", now)
	}
	b.insert(t, b.draft(domain.NotificationTagScanned, false), b.newScan(t), "", now)
	if err := a.store.MarkRead(ctx, a.userID, a.notifications(t)[0].ID, now); err != nil {
		t.Fatal(err)
	}
	n, err := a.store.MarkAllRead(ctx, a.userID, now)
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if n, _ := a.store.MarkAllRead(ctx, a.userID, now); n != 0 {
		t.Fatalf("second n=%d", n)
	}
	if c, _ := a.store.UnreadCount(ctx, a.userID); c != 0 {
		t.Fatalf("unread=%d", c)
	}
	if c, _ := b.store.UnreadCount(ctx, b.userID); c != 1 {
		t.Fatalf("b unread=%d", c)
	}
}
