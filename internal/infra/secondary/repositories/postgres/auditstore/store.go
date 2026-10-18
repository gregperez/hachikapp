package auditstore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	authBaseDomain "github.com/WilsonSayago/authBase/v4/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/domain"
	"github.com/gregperez/hachikapp-backend/internal/core/ports"
	"github.com/gregperez/hachikapp-backend/internal/infra/config/database"
)

const defaultPurgeBatchSize = 500

type Store struct {
	db             *database.PostgresPgx
	now            func() time.Time
	purgeBatchSize int
}

func New(db *database.PostgresPgx) *Store {
	return NewWithPurgeBatchSize(db, defaultPurgeBatchSize)
}

func NewWithPurgeBatchSize(db *database.PostgresPgx, purgeBatchSize int) *Store {
	if purgeBatchSize <= 0 {
		purgeBatchSize = defaultPurgeBatchSize
	}
	return &Store{db: db, now: time.Now, purgeBatchSize: purgeBatchSize}
}

var _ ports.AuditEventPort = (*Store)(nil)

func (s *Store) Insert(ctx context.Context, event domain.AuditEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if event.Action == "" || event.Category == "" || event.Outcome == "" {
		return fmt.Errorf("invalid audit event")
	}
	occurred := event.OccurredAt
	if occurred.IsZero() {
		occurred = s.now()
	}
	meta, err := json.Marshal(domain.SanitizeAuditMetadata(event.Metadata))
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}
	_, err = s.db.Exec(ctx, `
INSERT INTO public.audit_events
  (occurred_at, category, action, outcome, actor_id, subject_type, subject_id, request_id, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		occurred.UTC(),
		event.Category,
		event.Action,
		event.Outcome,
		domain.NormalizeAuditActor(event.ActorID),
		event.SubjectType,
		event.SubjectID,
		event.RequestID,
		meta,
	)
	if err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

func (s *Store) List(
	ctx context.Context,
	filter domain.AuditListFilter,
	request authBaseDomain.PageRequest,
) (authBaseDomain.Page[domain.AuditEvent], error) {
	if err := ctx.Err(); err != nil {
		return authBaseDomain.Page[domain.AuditEvent]{}, err
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 10
	}
	args := []any{}
	where := []string{}
	add := func(sql string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(sql, len(args)))
	}
	if filter.Action != "" {
		add("action = $%d", filter.Action)
	}
	if filter.Outcome != "" {
		add("outcome = $%d", filter.Outcome)
	}
	if filter.ActorID != "" {
		add("actor_id = $%d", filter.ActorID)
	}
	if filter.SubjectID != "" {
		add("subject_id = $%d", filter.SubjectID)
	}
	if !filter.From.IsZero() {
		add("occurred_at >= $%d", filter.From.UTC())
	}
	if !filter.To.IsZero() {
		add("occurred_at <= $%d", filter.To.UTC())
	}
	if request.After != nil {
		args = append(args, request.After.CreatedAt.UTC(), request.After.ID)
		where = append(where, fmt.Sprintf("(occurred_at, id) < ($%d, $%d)", len(args)-1, len(args)))
	}
	args = append(args, limit+1)
	query := `
SELECT id, occurred_at, category, action, outcome, actor_id, subject_type, subject_id, request_id, metadata
FROM public.audit_events`
	if len(where) > 0 {
		query += "\nWHERE " + strings.Join(where, " AND ")
	}
	query += fmt.Sprintf("\nORDER BY occurred_at DESC, id DESC\nLIMIT $%d", len(args))

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return authBaseDomain.Page[domain.AuditEvent]{}, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()

	items := make([]domain.AuditEvent, 0, limit)
	for rows.Next() {
		var item domain.AuditEvent
		var raw []byte
		if err := rows.Scan(
			&item.ID,
			&item.OccurredAt,
			&item.Category,
			&item.Action,
			&item.Outcome,
			&item.ActorID,
			&item.SubjectType,
			&item.SubjectID,
			&item.RequestID,
			&raw,
		); err != nil {
			return authBaseDomain.Page[domain.AuditEvent]{}, fmt.Errorf("scan audit event: %w", err)
		}
		meta := map[string]any{}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &meta)
		}
		item.Metadata = domain.SanitizeAuditMetadata(meta)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return authBaseDomain.Page[domain.AuditEvent]{}, fmt.Errorf("iterate audit events: %w", err)
	}
	hasNext := len(items) > limit
	if hasNext {
		items = items[:limit]
	}
	return authBaseDomain.Page[domain.AuditEvent]{Items: items, HasNext: hasNext}, nil
}

func (s *Store) PurgeBefore(ctx context.Context, before time.Time) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if before.IsZero() {
		return 0, fmt.Errorf("purge cutoff must not be zero")
	}
	tag, err := s.db.Exec(ctx, `
WITH expired AS (
	SELECT id
	FROM public.audit_events
	WHERE occurred_at < $1
	ORDER BY occurred_at
	LIMIT $2
	FOR UPDATE SKIP LOCKED
)
DELETE FROM public.audit_events AS events
USING expired
WHERE events.id = expired.id`, before.UTC(), s.purgeBatchSize)
	if err != nil {
		return 0, fmt.Errorf("purge audit events: %w", err)
	}
	return tag.RowsAffected(), nil
}
