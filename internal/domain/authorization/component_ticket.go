package authorization

import (
	"context"
	"errors"
	"time"
)

// ErrComponentVerifyTicketNotFound indicates a component verify ticket was not found.
var ErrComponentVerifyTicketNotFound = errors.New("component verify ticket not found")

// ComponentVerifyTicket stores the latest WeChat third-party platform verify ticket.
type ComponentVerifyTicket struct {
	ComponentAppID string
	Ticket         string
	ReceivedAt     time.Time
	UpdatedAt      time.Time
}

// ComponentVerifyTicketRepository persists component verify tickets.
type ComponentVerifyTicketRepository interface {
	SaveComponentVerifyTicket(ctx context.Context, ticket ComponentVerifyTicket) (ComponentVerifyTicket, error)
	GetComponentVerifyTicket(ctx context.Context, componentAppID string) (ComponentVerifyTicket, error)
}
