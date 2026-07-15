package application

import (
	"fmt"
	"strings"
)

// Actor is the server-resolved identity attached to one business mutation.
type Actor struct {
	UserID        string
	ActorType     ActorType
	AgentRecordID string
}

// Actor returns the safe mutation identity represented by this principal.
func (p Principal) Actor() Actor {
	actor := Actor{UserID: p.User.ID, ActorType: p.ActorType}
	if p.ActorType == ActorTypeAgentToken && p.Agent != nil {
		actor.AgentRecordID = p.Agent.ID
	}
	return actor
}

func normalizeActor(tenantID string, actor Actor) (Actor, error) {
	tenantID = strings.TrimSpace(tenantID)
	actor.UserID = strings.TrimSpace(actor.UserID)
	actor.AgentRecordID = strings.TrimSpace(actor.AgentRecordID)
	if actor.UserID == "" {
		actor.UserID = tenantID
	}
	if tenantID == "" || actor.UserID != tenantID {
		return Actor{}, fmt.Errorf("validate actor user: %w", ErrInvalidInput)
	}
	if actor.ActorType != ActorTypeAgentToken {
		actor.AgentRecordID = ""
	}
	return actor, nil
}
