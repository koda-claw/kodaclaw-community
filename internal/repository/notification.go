package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vanzheng/kodaclaw-community/internal/model"
)

var ErrNotificationNotFound = errors.New("notification not found")

type NotificationRepository interface {
	Create(ctx context.Context, n *model.Notification) error
	ListByUserID(ctx context.Context, userID uuid.UUID, page, pageSize int, onlyUnread bool) ([]model.Notification, int, int, error)
	MarkRead(ctx context.Context, userID, notificationID uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
}

type notificationRepo struct {
	pool *pgxpool.Pool
}

func NewNotificationRepository(pool *pgxpool.Pool) NotificationRepository {
	return &notificationRepo{pool: pool}
}

func (r *notificationRepo) Create(ctx context.Context, n *model.Notification) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO notifications (user_id, type, title, message, related_asset_id)
		 VALUES ($1, $2, $3, $4, $5)`,
		n.UserID, n.Type, n.Title, n.Message, n.RelatedAssetID)
	return err
}

func (r *notificationRepo) ListByUserID(ctx context.Context, userID uuid.UUID, page, pageSize int, onlyUnread bool) ([]model.Notification, int, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var total int
	if onlyUnread {
		if err := r.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = FALSE`, userID).Scan(&total); err != nil {
			return nil, 0, 0, err
		}
	} else {
		if err := r.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM notifications WHERE user_id = $1`, userID).Scan(&total); err != nil {
			return nil, 0, 0, err
		}
	}

	var unread int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = FALSE`, userID).Scan(&unread); err != nil {
		return nil, 0, 0, err
	}

	offset := (page - 1) * pageSize

	var rows pgx.Rows
	var err error
	if onlyUnread {
		rows, err = r.pool.Query(ctx,
			`SELECT id, user_id, type, title, message, related_asset_id, is_read, created_at
			 FROM notifications WHERE user_id = $1 AND is_read = FALSE
			 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			userID, pageSize, offset)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT id, user_id, type, title, message, related_asset_id, is_read, created_at
			 FROM notifications WHERE user_id = $1
			 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			userID, pageSize, offset)
	}
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()

	var items []model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Message, &n.RelatedAssetID, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, 0, 0, err
		}
		items = append(items, n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, 0, err
	}

	return items, total, unread, nil
}

func (r *notificationRepo) MarkRead(ctx context.Context, userID, notificationID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE notifications SET is_read = TRUE WHERE id = $1 AND user_id = $2`,
		notificationID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotificationNotFound
	}
	return nil
}

func (r *notificationRepo) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE notifications SET is_read = TRUE WHERE user_id = $1`, userID)
	return err
}
