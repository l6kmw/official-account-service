package agentaudit

import (
	"context"
	"time"
)

// Action identifies one audited business mutation.
type Action string

const (
	ActionCreateArticle  Action = "create_article"
	ActionUpdateArticle  Action = "update_article"
	ActionPublishArticle Action = "publish_article"
	ActionDeleteArticle  Action = "delete_article"
)

// ResourceType identifies the affected business resource.
type ResourceType string

const (
	ResourceArticle ResourceType = "article"
)

// Entry contains only safe mutation metadata. Request payloads and credentials have no representation here.
type Entry struct {
	ID            int64
	UserID        string
	AgentRecordID string
	Action        Action
	ResourceType  ResourceType
	ResourceID    string
	CreatedAt     time.Time
}

// Filter narrows one user's append-only audit history.
type Filter struct {
	AgentRecordID string
	Action        Action
	ResourceType  ResourceType
	Limit         int
}

// Repository stores and lists append-only Agent audit entries.
type Repository interface {
	Append(ctx context.Context, entry Entry) (Entry, error)
	List(ctx context.Context, userID string, filter Filter) ([]Entry, error)
}
