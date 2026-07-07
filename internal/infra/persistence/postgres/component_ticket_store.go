package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"official-account-service/internal/domain/authorization"
)

// SaveComponentVerifyTicket stores the latest component verify ticket.
func (s *Store) SaveComponentVerifyTicket(ctx context.Context, ticket authorization.ComponentVerifyTicket) (authorization.ComponentVerifyTicket, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO wechat_component_verify_ticket (component_app_id, verify_ticket, received_at, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (component_app_id) DO UPDATE SET
			verify_ticket = EXCLUDED.verify_ticket,
			received_at = EXCLUDED.received_at,
			updated_at = NOW()
		RETURNING component_app_id, verify_ticket, received_at, updated_at`,
		ticket.ComponentAppID, ticket.Ticket, ticket.ReceivedAt)
	saved, err := scanComponentVerifyTicketRow(row)
	if err != nil {
		return authorization.ComponentVerifyTicket{}, fmt.Errorf("save component verify ticket: %w", err)
	}
	return saved, nil
}

// GetComponentVerifyTicket returns the latest component verify ticket.
func (s *Store) GetComponentVerifyTicket(ctx context.Context, componentAppID string) (authorization.ComponentVerifyTicket, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT component_app_id, verify_ticket, received_at, updated_at
		FROM wechat_component_verify_ticket
		WHERE component_app_id = $1`, componentAppID)
	ticket, err := scanComponentVerifyTicketRow(row)
	if err != nil {
		return authorization.ComponentVerifyTicket{}, mapComponentVerifyTicketError("get component verify ticket", err)
	}
	return ticket, nil
}

func scanComponentVerifyTicketRow(row *sql.Row) (authorization.ComponentVerifyTicket, error) {
	var ticket authorization.ComponentVerifyTicket
	if err := row.Scan(&ticket.ComponentAppID, &ticket.Ticket, &ticket.ReceivedAt, &ticket.UpdatedAt); err != nil {
		return authorization.ComponentVerifyTicket{}, fmt.Errorf("scan component verify ticket: %w", err)
	}
	return ticket, nil
}

func mapComponentVerifyTicketError(action string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", action, authorization.ErrComponentVerifyTicketNotFound)
	}
	return fmt.Errorf("%s: %w", action, err)
}
