package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
)

var ErrSagaNotFound = errors.New("saga instance not found")

type Repository struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Repository, error) {
	if databaseURL == "" {
		return nil, errors.New("database URL is required")
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	return &Repository{pool: pool}, nil
}

func (r *Repository) Close() {
	if r == nil || r.pool == nil {
		return
	}
	r.pool.Close()
}

func (r *Repository) Ping(ctx context.Context) error {
	if r == nil || r.pool == nil {
		return errors.New("repository is not initialized")
	}
	return r.pool.Ping(ctx)
}

func (r *Repository) CreateSagaInstance(ctx context.Context, instance domain.SagaInstance) error {
	if r == nil || r.pool == nil {
		return errors.New("repository is not initialized")
	}
	if instance.ID == "" {
		return errors.New("saga instance ID is required")
	}
	if instance.TemplateID == "" {
		return errors.New("template ID is required")
	}
	if !instance.Status.Valid() {
		return fmt.Errorf("invalid saga status: %q", instance.Status)
	}

	const query = `
		INSERT INTO saga_instances (
			id,
			template_id,
			status,
			input_json,
			idempotency_key,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.pool.Exec(
		ctx,
		query,
		instance.ID,
		instance.TemplateID,
		instance.Status,
		instance.InputJSON,
		instance.IdempotencyKey,
		instance.CreatedAt,
		instance.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert saga instance: %w", err)
	}

	return nil
}

func (r *Repository) CreateSagaInstanceWithOutbox(ctx context.Context, instance domain.SagaInstance, event domain.OutboxEvent) error {
	if r == nil || r.pool == nil {
		return errors.New("repository is not initialized")
	}
	if instance.ID == "" {
		return errors.New("saga instance ID is required")
	}
	if instance.TemplateID == "" {
		return errors.New("template ID is required")
	}
	if !instance.Status.Valid() {
		return fmt.Errorf("invalid saga status: %q", instance.Status)
	}
	if event.AggregateID == "" || event.AggregateType == "" || event.EventType == "" || event.DedupeKey == "" {
		return errors.New("outbox event fields are required")
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	const insertSagaQuery = `
		INSERT INTO saga_instances (
			id,
			template_id,
			status,
			input_json,
			idempotency_key,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	if _, err := tx.Exec(
		ctx,
		insertSagaQuery,
		instance.ID,
		instance.TemplateID,
		instance.Status,
		instance.InputJSON,
		instance.IdempotencyKey,
		instance.CreatedAt,
		instance.UpdatedAt,
	); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("insert saga instance: %w", err)
	}

	const insertOutboxQuery = `
		INSERT INTO outbox_events (
			aggregate_type,
			aggregate_id,
			event_type,
			payload_json,
			dedupe_key,
			status,
			attempts,
			next_attempt_at,
			lease_owner,
			lease_until,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	if _, err := tx.Exec(
		ctx,
		insertOutboxQuery,
		event.AggregateType,
		event.AggregateID,
		event.EventType,
		event.PayloadJSON,
		event.DedupeKey,
		event.Status,
		event.Attempts,
		event.NextAttemptAt,
		event.LeaseOwner,
		event.LeaseUntil,
		event.CreatedAt,
		event.UpdatedAt,
	); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("insert outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create saga with outbox: %w", err)
	}

	return nil
}

func (r *Repository) GetSagaInstance(ctx context.Context, id string) (domain.SagaInstance, error) {
	if r == nil || r.pool == nil {
		return domain.SagaInstance{}, errors.New("repository is not initialized")
	}
	if id == "" {
		return domain.SagaInstance{}, errors.New("saga instance ID is required")
	}

	const query = `
		SELECT
			id,
			template_id,
			status,
			input_json,
			idempotency_key,
			created_at,
			updated_at
		FROM saga_instances
		WHERE id = $1
	`

	var instance domain.SagaInstance
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&instance.ID,
		&instance.TemplateID,
		&instance.Status,
		&instance.InputJSON,
		&instance.IdempotencyKey,
		&instance.CreatedAt,
		&instance.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SagaInstance{}, ErrSagaNotFound
		}
		return domain.SagaInstance{}, fmt.Errorf("query saga instance: %w", err)
	}

	return instance, nil
}

func (r *Repository) UpdateSagaStatus(ctx context.Context, id string, status domain.SagaStatus) error {
	if r == nil || r.pool == nil {
		return errors.New("repository is not initialized")
	}
	if id == "" {
		return errors.New("saga instance ID is required")
	}
	if !status.Valid() {
		return fmt.Errorf("invalid saga status: %q", status)
	}

	const query = `
		UPDATE saga_instances
		SET status = $2, updated_at = NOW()
		WHERE id = $1
	`

	tag, err := r.pool.Exec(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("update saga status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSagaNotFound
	}

	return nil
}

func (r *Repository) ClaimDispatchableOutboxEvents(ctx context.Context, now time.Time, leaseOwner string, leaseUntil time.Time, limit int) ([]domain.OutboxEvent, error) {
	if r == nil || r.pool == nil {
		return nil, errors.New("repository is not initialized")
	}
	if leaseOwner == "" {
		return nil, errors.New("lease owner is required")
	}
	if limit <= 0 {
		limit = 100
	}

	const query = `
		WITH candidates AS (
			SELECT dedupe_key
			FROM outbox_events
			WHERE (
					status = 'pending'
					OR (status = 'failed' AND next_attempt_at IS NOT NULL AND next_attempt_at <= $1)
				)
				AND (
					lease_until IS NULL
					OR lease_until <= $1
					OR lease_owner = $2
				)
			ORDER BY created_at ASC, dedupe_key ASC
			LIMIT $4
			FOR UPDATE SKIP LOCKED
		)
		UPDATE outbox_events o
		SET lease_owner = $2, lease_until = $3, updated_at = NOW()
		FROM candidates c
		WHERE o.dedupe_key = c.dedupe_key
		RETURNING
			o.aggregate_type,
			o.aggregate_id,
			o.event_type,
			o.payload_json,
			o.dedupe_key,
			o.status,
			o.attempts,
			o.next_attempt_at,
			o.lease_owner,
			o.lease_until,
			o.created_at,
			o.updated_at
	`

	rows, err := r.pool.Query(ctx, query, now, leaseOwner, leaseUntil, limit)
	if err != nil {
		return nil, fmt.Errorf("claim dispatchable outbox events: %w", err)
	}
	defer rows.Close()

	var events []domain.OutboxEvent
	for rows.Next() {
		var event domain.OutboxEvent
		if err := rows.Scan(
			&event.AggregateType,
			&event.AggregateID,
			&event.EventType,
			&event.PayloadJSON,
			&event.DedupeKey,
			&event.Status,
			&event.Attempts,
			&event.NextAttemptAt,
			&event.LeaseOwner,
			&event.LeaseUntil,
			&event.CreatedAt,
			&event.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
	}

	return events, nil
}

func (r *Repository) UpdateOutboxEventDelivery(ctx context.Context, dedupeKey string, status string, attempts int, nextAttemptAt *time.Time, leaseOwner string) error {
	if r == nil || r.pool == nil {
		return errors.New("repository is not initialized")
	}
	if dedupeKey == "" {
		return errors.New("dedupe key is required")
	}
	if status == "" {
		return errors.New("status is required")
	}

	const query = `
		UPDATE outbox_events
		SET status = $2, attempts = $3, next_attempt_at = $4, lease_owner = NULL, lease_until = NULL, updated_at = NOW()
		WHERE dedupe_key = $1
		  AND ($5 = '' OR lease_owner = $5)
	`

	tag, err := r.pool.Exec(ctx, query, dedupeKey, status, attempts, nextAttemptAt, leaseOwner)
	if err != nil {
		return fmt.Errorf("update outbox event status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSagaNotFound
	}

	return nil
}
