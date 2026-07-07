package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"official-account-service/internal/domain/wechatcallback"
)

// SaveCallbackEvent stores callback raw payload and dedupe metadata.
func (s *Store) SaveCallbackEvent(ctx context.Context, event wechatcallback.Event) (wechatcallback.Event, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO wechat_callback_event (tenant_id, component_app_id, authorizer_app_id, event_type, event_key, raw_body, received_at, retain_until)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, tenant_id, component_app_id, authorizer_app_id, event_type, event_key, raw_body, received_at, retain_until, created_at`,
		event.TenantID, event.ComponentAppID, event.AuthorizerAppID, event.EventType, event.EventKey, event.RawBody, event.ReceivedAt, event.RetainUntil)
	saved, err := scanCallbackEventRow(row)
	if err != nil {
		if isUniqueViolation(err) {
			return wechatcallback.Event{}, fmt.Errorf("save callback event duplicate: %w", wechatcallback.ErrDuplicate)
		}
		return wechatcallback.Event{}, fmt.Errorf("save callback event: %w", err)
	}
	return saved, nil
}

// GetCallbackEventByKey returns one callback event by tenant-scoped dedupe key.
func (s *Store) GetCallbackEventByKey(ctx context.Context, tenantID string, eventType string, eventKey string) (wechatcallback.Event, error) {
	row := s.db.QueryRowContext(ctx, callbackEventSelectSQL+` WHERE tenant_id = $1 AND event_type = $2 AND event_key = $3`, tenantID, eventType, eventKey)
	event, err := scanCallbackEventRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wechatcallback.Event{}, fmt.Errorf("get callback event: %w", wechatcallback.ErrNotFound)
		}
		return wechatcallback.Event{}, fmt.Errorf("get callback event: %w", err)
	}
	return event, nil
}

const callbackEventSelectSQL = `
	SELECT id, tenant_id, component_app_id, authorizer_app_id, event_type, event_key, raw_body, received_at, retain_until, created_at
	FROM wechat_callback_event`

func scanCallbackEventRow(row *sql.Row) (wechatcallback.Event, error) {
	var event wechatcallback.Event
	if err := row.Scan(&event.ID, &event.TenantID, &event.ComponentAppID, &event.AuthorizerAppID, &event.EventType, &event.EventKey, &event.RawBody, &event.ReceivedAt, &event.RetainUntil, &event.CreatedAt); err != nil {
		return wechatcallback.Event{}, err
	}
	return event, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
