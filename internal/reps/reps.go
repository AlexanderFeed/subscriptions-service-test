package reps

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"subscriptions-service/internal/models"
)

var ErrNotFound = errors.New("subscription not found")

type SubscriptionRepository interface {
	Create(ctx context.Context, input models.CreateSubscriptionInput) (*models.Subscription, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Subscription, error)
	List(ctx context.Context, f models.ListFilter) ([]*models.Subscription, int, error)
	Update(ctx context.Context, id uuid.UUID, input models.UpdateSubscriptionInput) (*models.Subscription, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListForCost(ctx context.Context, f models.CostFilter) ([]*models.Subscription, error)
}

type subscriptionRepository struct {
	db *pgxpool.Pool
}

func NewSubscriptionRepository(db *pgxpool.Pool) SubscriptionRepository {
	return &subscriptionRepository{db: db}
}

func (r *subscriptionRepository) Create(ctx context.Context, in models.CreateSubscriptionInput) (*models.Subscription, error) {
	const q = `
		INSERT INTO subscriptions (service_name, price, user_id, start_date, end_date)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, service_name, price, user_id, start_date, end_date, created_at, updated_at`

	row := r.db.QueryRow(ctx, q, in.ServiceName, in.Price, in.UserID, in.StartDate, in.EndDate)
	return scanRow(row)
}

func (r *subscriptionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Subscription, error) {
	const q = `
		SELECT id, service_name, price, user_id, start_date, end_date, created_at, updated_at
		FROM subscriptions WHERE id = $1`

	sub, err := scanRow(r.db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return sub, err
}

func (r *subscriptionRepository) List(ctx context.Context, f models.ListFilter) ([]*models.Subscription, int, error) {
	args := []any{}
	conds := []string{}
	idx := 1

	if f.UserID != nil {
		conds = append(conds, fmt.Sprintf("user_id = $%d", idx))
		args = append(args, *f.UserID)
		idx++
	}
	if f.ServiceName != nil {
		conds = append(conds, fmt.Sprintf("service_name ILIKE $%d", idx))
		args = append(args, "%"+*f.ServiceName+"%")
		idx++
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	countQ := fmt.Sprintf("SELECT COUNT(*) FROM subscriptions %s", where)
	if err := r.db.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count: %w", err)
	}

	dataQ := fmt.Sprintf(`
		SELECT id, service_name, price, user_id, start_date, end_date, created_at, updated_at
		FROM subscriptions %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, idx, idx+1)

	args = append(args, f.Limit, f.Offset)
	rows, err := r.db.Query(ctx, dataQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list: %w", err)
	}
	defer rows.Close()

	var subs []*models.Subscription
	for rows.Next() {
		var s models.Subscription
		var endDate *time.Time
		if err := rows.Scan(&s.ID, &s.ServiceName, &s.Price, &s.UserID,
			&s.StartDate, &endDate, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, 0, err
		}
		s.EndDate = endDate
		subs = append(subs, &s)
	}
	return subs, total, rows.Err()
}

func (r *subscriptionRepository) Update(ctx context.Context, id uuid.UUID, in models.UpdateSubscriptionInput) (*models.Subscription, error) {
	sets := []string{}
	args := []any{}
	idx := 1

	if in.ServiceName != nil {
		sets = append(sets, fmt.Sprintf("service_name = $%d", idx))
		args = append(args, *in.ServiceName)
		idx++
	}
	if in.Price != nil {
		sets = append(sets, fmt.Sprintf("price = $%d", idx))
		args = append(args, *in.Price)
		idx++
	}
	if in.StartDate != nil {
		sets = append(sets, fmt.Sprintf("start_date = $%d", idx))
		args = append(args, *in.StartDate)
		idx++
	}
	if in.EndDate != nil {
		sets = append(sets, fmt.Sprintf("end_date = $%d", idx))
		args = append(args, *in.EndDate)
		idx++
	}

	if len(sets) == 0 {
		return r.GetByID(ctx, id)
	}

	sets = append(sets, fmt.Sprintf("updated_at = $%d", idx))
	args = append(args, time.Now().UTC())
	idx++
	args = append(args, id)

	q := fmt.Sprintf(`
		UPDATE subscriptions SET %s WHERE id = $%d
		RETURNING id, service_name, price, user_id, start_date, end_date, created_at, updated_at`,
		strings.Join(sets, ", "), idx)

	sub, err := scanRow(r.db.QueryRow(ctx, q, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return sub, err
}

func (r *subscriptionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	cmd, err := r.db.Exec(ctx, "DELETE FROM subscriptions WHERE id = $1", id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *subscriptionRepository) ListForCost(ctx context.Context, f models.CostFilter) ([]*models.Subscription, error) {
	args := []any{f.PeriodStart, f.PeriodEnd}
	conds := []string{
		"start_date <= $2",
		"(end_date IS NULL OR end_date >= $1)",
	}
	idx := 3

	if f.UserID != nil {
		conds = append(conds, fmt.Sprintf("user_id = $%d", idx))
		args = append(args, *f.UserID)
		idx++
	}
	if f.ServiceName != nil {
		conds = append(conds, fmt.Sprintf("service_name ILIKE $%d", idx))
		args = append(args, "%"+*f.ServiceName+"%")
	}

	q := fmt.Sprintf(`
		SELECT id, service_name, price, user_id, start_date, end_date, created_at, updated_at
		FROM subscriptions WHERE %s`, strings.Join(conds, " AND "))

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*models.Subscription
	for rows.Next() {
		var s models.Subscription
		var endDate *time.Time
		if err := rows.Scan(&s.ID, &s.ServiceName, &s.Price, &s.UserID,
			&s.StartDate, &endDate, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		s.EndDate = endDate
		subs = append(subs, &s)
	}
	return subs, rows.Err()
}

// scanRow используется для QueryRow (одна строка)
func scanRow(row pgx.Row) (*models.Subscription, error) {
	var s models.Subscription
	var endDate *time.Time
	err := row.Scan(&s.ID, &s.ServiceName, &s.Price, &s.UserID,
		&s.StartDate, &endDate, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.EndDate = endDate
	return &s, nil
}