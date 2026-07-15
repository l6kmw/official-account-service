package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"official-account-service/internal/domain/agentaudit"
	"official-account-service/internal/domain/article"
	"official-account-service/internal/domain/authorization"
)

// ArticleService manages platform article drafts.
type ArticleService struct {
	articles        article.Repository
	authorizers     authorization.Repository
	audits          agentaudit.Repository
	auditedArticles article.AuditedMutationRepository
}

// NewArticleService constructs an ArticleService.
func NewArticleService(articles article.Repository) *ArticleService {
	return &ArticleService{articles: articles}
}

// NewArticleServiceWithAuthorizerRepository constructs an ArticleService that validates authorizer ownership.
func NewArticleServiceWithAuthorizerRepository(articles article.Repository, authorizers authorization.Repository) *ArticleService {
	return &ArticleService{articles: articles, authorizers: authorizers}
}

// NewArticleServiceWithRepositories constructs an ArticleService with ownership validation and mutation auditing.
func NewArticleServiceWithRepositories(articles article.Repository, authorizers authorization.Repository, audits agentaudit.Repository) *ArticleService {
	service := &ArticleService{articles: articles, authorizers: authorizers, audits: audits}
	if audited, ok := audits.(article.AuditedMutationRepository); ok {
		service.auditedArticles = audited
	}
	return service
}

// CreateArticleInput contains fields for creating a platform article draft.
type CreateArticleInput struct {
	TenantID     string
	AuthorizerID int64
	Title        string
	Author       string
	Digest       string
	ContentHTML  string
	Actor        Actor
}

// UpdateArticleInput contains editable platform article draft fields.
type UpdateArticleInput struct {
	TenantID          string
	ID                int64
	Title             string
	Author            string
	Digest            string
	ContentHTML       string
	CoverMediaAssetID int64
	Version           int64
	Actor             Actor
}

// DeleteArticleInput identifies one actor-attributed article deletion.
type DeleteArticleInput struct {
	TenantID string
	ID       int64
	Actor    Actor
}

// CreateArticle creates a tenant-scoped platform article draft.
func (s *ArticleService) CreateArticle(ctx context.Context, input CreateArticleInput) (article.Article, error) {
	if err := s.validateReady(); err != nil {
		return article.Article{}, err
	}
	if strings.TrimSpace(input.TenantID) == "" {
		return article.Article{}, fmt.Errorf("validate article tenant id: %w", ErrInvalidInput)
	}
	if input.AuthorizerID <= 0 {
		return article.Article{}, fmt.Errorf("validate article authorizer id: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(input.Title) == "" {
		return article.Article{}, fmt.Errorf("validate article title: %w", ErrInvalidInput)
	}
	actor, err := normalizeActor(input.TenantID, input.Actor)
	if err != nil {
		return article.Article{}, err
	}
	if err := s.validateAuthorizer(ctx, input.TenantID, input.AuthorizerID); err != nil {
		return article.Article{}, err
	}
	draft := article.Article{
		TenantID:         input.TenantID,
		AuthorizerID:     input.AuthorizerID,
		Title:            input.Title,
		Author:           input.Author,
		Digest:           input.Digest,
		ContentHTML:      input.ContentHTML,
		Status:           article.StatusDraft,
		CreatedByAgentID: actor.AgentRecordID,
		UpdatedByAgentID: actor.AgentRecordID,
	}
	var created article.Article
	if s.audits != nil {
		if s.auditedArticles == nil {
			return article.Article{}, fmt.Errorf("create article audit repository: %w", ErrNotImplemented)
		}
		created, err = s.auditedArticles.CreateWithAudit(ctx, input.TenantID, draft, articleAuditEntry(actor, agentaudit.ActionCreateArticle, 0))
		if err != nil {
			return article.Article{}, fmt.Errorf("create article with audit: %w", err)
		}
	} else {
		created, err = s.articles.Create(ctx, input.TenantID, draft)
		if err != nil {
			return article.Article{}, fmt.Errorf("create article: %w", err)
		}
	}
	return created, nil
}

// GetArticle returns one tenant-scoped article draft.
func (s *ArticleService) GetArticle(ctx context.Context, tenantID string, id int64) (article.Article, error) {
	if err := s.validateArticleID(tenantID, id); err != nil {
		return article.Article{}, err
	}
	draft, err := s.articles.Get(ctx, tenantID, id)
	if err != nil {
		return article.Article{}, s.wrapArticleReadError("get article", err)
	}
	return draft, nil
}

// ListArticles returns tenant-scoped article drafts.
func (s *ArticleService) ListArticles(ctx context.Context, tenantID string) ([]article.Article, error) {
	if err := s.validateReady(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(tenantID) == "" {
		return nil, fmt.Errorf("validate article tenant id: %w", ErrInvalidInput)
	}
	articles, err := s.articles.List(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list articles: %w", err)
	}
	return articles, nil
}

// ListArticlesByAgent returns articles created by one Agent under the current user.
func (s *ArticleService) ListArticlesByAgent(ctx context.Context, tenantID string, agentRecordID string) ([]article.Article, error) {
	if err := s.validateReady(); err != nil {
		return nil, err
	}
	tenantID = strings.TrimSpace(tenantID)
	agentRecordID = strings.TrimSpace(agentRecordID)
	if tenantID == "" || agentRecordID == "" {
		return nil, fmt.Errorf("validate article agent filter: %w", ErrInvalidInput)
	}
	repository, ok := s.articles.(article.CreatedByAgentRepository)
	if !ok {
		return nil, fmt.Errorf("list articles by agent: %w", ErrNotImplemented)
	}
	items, err := repository.ListByCreatedByAgent(ctx, tenantID, agentRecordID)
	if err != nil {
		return nil, fmt.Errorf("list articles by agent: %w", err)
	}
	return items, nil
}

// UpdateArticle updates editable fields on one tenant-scoped article draft.
func (s *ArticleService) UpdateArticle(ctx context.Context, input UpdateArticleInput) (article.Article, error) {
	if err := s.validateArticleID(input.TenantID, input.ID); err != nil {
		return article.Article{}, err
	}
	if strings.TrimSpace(input.Title) == "" {
		return article.Article{}, fmt.Errorf("validate article title: %w", ErrInvalidInput)
	}
	actor, err := normalizeActor(input.TenantID, input.Actor)
	if err != nil {
		return article.Article{}, err
	}
	current, err := s.articles.Get(ctx, input.TenantID, input.ID)
	if err != nil {
		return article.Article{}, s.wrapArticleReadError("get article for update", err)
	}
	if input.Version > 0 && input.Version != current.Version {
		return article.Article{}, fmt.Errorf("validate article version: %w", ErrConflict)
	}
	current.Title = input.Title
	current.Author = input.Author
	current.Digest = input.Digest
	current.ContentHTML = input.ContentHTML
	current.CoverMediaAssetID = input.CoverMediaAssetID
	current.UpdatedByAgentID = actor.AgentRecordID
	var updated article.Article
	if s.audits != nil {
		if s.auditedArticles == nil {
			return article.Article{}, fmt.Errorf("update article audit repository: %w", ErrNotImplemented)
		}
		updated, err = s.auditedArticles.UpdateWithAudit(ctx, input.TenantID, current, articleAuditEntry(actor, agentaudit.ActionUpdateArticle, current.ID))
	} else {
		updated, err = s.articles.Update(ctx, input.TenantID, current)
	}
	if err != nil {
		return article.Article{}, s.wrapArticleReadError("update article", err)
	}
	return updated, nil
}

// DeleteArticle deletes one tenant-scoped article draft.
func (s *ArticleService) DeleteArticle(ctx context.Context, tenantID string, id int64) error {
	return s.DeleteArticleWithActor(ctx, DeleteArticleInput{TenantID: tenantID, ID: id})
}

// DeleteArticleWithActor deletes one article and appends safe actor metadata.
func (s *ArticleService) DeleteArticleWithActor(ctx context.Context, input DeleteArticleInput) error {
	if err := s.validateArticleID(input.TenantID, input.ID); err != nil {
		return err
	}
	actor, err := normalizeActor(input.TenantID, input.Actor)
	if err != nil {
		return err
	}
	if s.audits != nil {
		if s.auditedArticles == nil {
			return fmt.Errorf("delete article audit repository: %w", ErrNotImplemented)
		}
		err = s.auditedArticles.DeleteWithAudit(ctx, input.TenantID, input.ID, articleAuditEntry(actor, agentaudit.ActionDeleteArticle, input.ID))
	} else {
		err = s.articles.Delete(ctx, input.TenantID, input.ID)
	}
	if err != nil {
		return s.wrapArticleReadError("delete article", err)
	}
	return nil
}

func (s *ArticleService) validateReady() error {
	if s == nil || s.articles == nil {
		return fmt.Errorf("validate article service repository: %w", ErrInvalidInput)
	}
	return nil
}

func (s *ArticleService) validateArticleID(tenantID string, id int64) error {
	if err := s.validateReady(); err != nil {
		return err
	}
	if strings.TrimSpace(tenantID) == "" {
		return fmt.Errorf("validate article tenant id: %w", ErrInvalidInput)
	}
	if id <= 0 {
		return fmt.Errorf("validate article id: %w", ErrInvalidInput)
	}
	return nil
}

func (s *ArticleService) wrapArticleReadError(action string, err error) error {
	if errors.Is(err, article.ErrNotFound) {
		return fmt.Errorf("%s: %w", action, ErrNotFound)
	}
	if errors.Is(err, article.ErrVersionConflict) {
		return fmt.Errorf("%s: %w", action, ErrConflict)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func articleAuditEntry(actor Actor, action agentaudit.Action, articleID int64) agentaudit.Entry {
	resourceID := ""
	if articleID > 0 {
		resourceID = strconv.FormatInt(articleID, 10)
	}
	return agentaudit.Entry{
		UserID: actor.UserID, AgentRecordID: actor.AgentRecordID, Action: action,
		ResourceType: agentaudit.ResourceArticle, ResourceID: resourceID,
	}
}

func (s *ArticleService) validateAuthorizer(ctx context.Context, tenantID string, authorizerID int64) error {
	if s.authorizers == nil {
		return nil
	}
	account, err := s.authorizers.GetAccount(ctx, tenantID, authorizerID)
	if errors.Is(err, authorization.ErrNotFound) {
		return fmt.Errorf("validate article authorizer: %w", ErrInvalidInput)
	}
	if err != nil {
		return fmt.Errorf("get article authorizer: %w", err)
	}
	if account.Status != authorization.AccountStatusActive {
		return fmt.Errorf("validate article authorizer status: %w", ErrInvalidInput)
	}
	return nil
}
