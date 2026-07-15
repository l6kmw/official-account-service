package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"official-account-service/internal/application"
	"official-account-service/internal/domain/authorization"
	"official-account-service/internal/domain/identity"
	"official-account-service/internal/domain/material"
	"official-account-service/internal/infra/persistence/memory"
)

func TestAgentArticleRoutesAttributeFilterAuditAndRejectStaleVersion(t *testing.T) {
	ctx := context.Background()
	store := memory.NewStore(fixedRouteTime)
	for _, userID := range []string{"user-1", "user-2"} {
		_, err := store.SaveUser(ctx, identity.User{
			ID: userID, Username: userID, PasswordHash: "hash", Role: identity.RoleUser, Status: identity.StatusActive,
		})
		require.NoError(t, err)
	}
	account, err := store.SaveAccount(ctx, "user-1", authorization.Account{
		AppID: "wx-user-1", Name: "User 1", Status: authorization.AccountStatusActive,
	})
	require.NoError(t, err)

	identities := application.NewIdentityService(store)
	articles := application.NewArticleServiceWithRepositories(store, store, store)
	publishes := application.NewPublishServiceWithPublisher(
		store, store, store, &routeFakePublisher{draftMediaID: "agent-draft", publishID: "agent-publish"},
		routeFakePublishTokenProvider{}, "wx-component", fixedRouteTime,
	).WithAuditRepository(store)
	router := NewRouter(Dependencies{
		Logger: zap.NewNop(), AdminAPIKey: "admin-key", AdminUserID: "admin-user",
		Identity: identities, Articles: articles, Publishes: publishes, AgentAudit: application.NewAgentAuditService(store),
	})
	agentA := createAgentThroughAPI(t, router, "user-1", `{"agent_id":"writer-a","name":"Writer A"}`)
	agentB := createAgentThroughAPI(t, router, "user-1", `{"agent_id":"writer-b","name":"Writer B"}`)
	otherUserAgent := createAgentThroughAPI(t, router, "user-2", `{"agent_id":"other","name":"Other"}`)

	createdResponse := doJSONWithBearer(t, router, http.MethodPost, "/api/v1/articles",
		fmt.Sprintf(`{"authorizer_id":%d,"title":"Agent article","content_html":"<p>sensitive-body</p>"}`, account.ID),
		"forged-user", agentA.Token)
	require.Equal(t, http.StatusCreated, createdResponse.Code, createdResponse.Body.String())
	var created articleResponse
	require.NoError(t, json.Unmarshal(createdResponse.Body.Bytes(), &created))
	require.Equal(t, agentA.Agent.ID, created.CreatedByAgentID)
	require.Equal(t, agentA.Agent.ID, created.UpdatedByAgentID)
	require.Equal(t, int64(1), created.Version)
	require.Equal(t, "user-1", created.TenantID)

	agentsResponse := doJSONWithBearer(t, router, http.MethodGet, "/api/v1/agents", ``, "user-2", agentB.Token)
	require.Equal(t, http.StatusOK, agentsResponse.Code)
	require.Contains(t, agentsResponse.Body.String(), agentA.Agent.ID)
	require.Contains(t, agentsResponse.Body.String(), agentB.Agent.ID)
	require.NotContains(t, agentsResponse.Body.String(), otherUserAgent.Agent.ID)
	for _, forbidden := range []string{"api_token_hash", "api_token_hint", "api_token_configured", agentA.Token, agentB.Token} {
		require.NotContains(t, agentsResponse.Body.String(), forbidden)
	}

	cover, err := store.CreateMaterial(ctx, "user-1", material.Asset{
		AuthorizerID: account.ID, ArticleID: created.ID, Usage: material.UsageCover, MediaID: "agent-cover",
	})
	require.NoError(t, err)
	updateBody := fmt.Sprintf(`{"title":"Updated by B","author":"","digest":"","content_html":"<p>updated-sensitive-body</p>","cover_media_asset_id":%d,"version":%d}`, cover.ID, created.Version)
	updatedResponse := doJSONWithBearer(t, router, http.MethodPut, fmt.Sprintf("/api/v1/articles/%d", created.ID), updateBody, "user-2", agentB.Token)
	require.Equal(t, http.StatusOK, updatedResponse.Code, updatedResponse.Body.String())
	var updated articleResponse
	require.NoError(t, json.Unmarshal(updatedResponse.Body.Bytes(), &updated))
	require.Equal(t, agentA.Agent.ID, updated.CreatedByAgentID)
	require.Equal(t, agentB.Agent.ID, updated.UpdatedByAgentID)
	require.Equal(t, int64(2), updated.Version)

	staleResponse := doJSONWithBearer(t, router, http.MethodPut, fmt.Sprintf("/api/v1/articles/%d", created.ID), updateBody, "user-1", agentA.Token)
	require.Equal(t, http.StatusConflict, staleResponse.Code)
	require.Contains(t, staleResponse.Body.String(), "article_version_conflict")
	require.Contains(t, staleResponse.Body.String(), "fetch the latest version")

	createdByA := doJSONWithBearer(t, router, http.MethodGet, "/api/v1/articles?agent_record_id="+agentA.Agent.ID, ``, "user-2", agentB.Token)
	require.Equal(t, http.StatusOK, createdByA.Code)
	require.Contains(t, createdByA.Body.String(), `"title":"Updated by B"`)
	createdByB := doJSONWithBearer(t, router, http.MethodGet, "/api/v1/articles?agent_record_id="+agentB.Agent.ID, ``, "user-1", agentA.Token)
	require.Equal(t, http.StatusOK, createdByB.Code)
	require.JSONEq(t, `{"items":[]}`, createdByB.Body.String())

	published := doJSONWithBearer(t, router, http.MethodPost, fmt.Sprintf("/api/v1/articles/%d/publish", created.ID), ``, "user-2", agentB.Token)
	require.Equal(t, http.StatusCreated, published.Code, published.Body.String())
	publishesByA := doJSONWithBearer(t, router, http.MethodGet, "/api/v1/publish-records?agent_record_id="+agentA.Agent.ID, ``, "user-2", agentB.Token)
	require.Equal(t, http.StatusOK, publishesByA.Code)
	require.Contains(t, publishesByA.Body.String(), `"wechat_publish_id":"agent-publish"`)
	require.Contains(t, publishesByA.Body.String(), `"article_created_by_agent_id":"`+agentA.Agent.ID+`"`)

	deleteCandidateResponse := doJSONWithBearer(t, router, http.MethodPost, "/api/v1/articles",
		fmt.Sprintf(`{"authorizer_id":%d,"title":"Delete me"}`, account.ID), "", agentA.Token)
	require.Equal(t, http.StatusCreated, deleteCandidateResponse.Code)
	var deleteCandidate articleResponse
	require.NoError(t, json.Unmarshal(deleteCandidateResponse.Body.Bytes(), &deleteCandidate))
	deleted := doJSONWithBearer(t, router, http.MethodDelete, fmt.Sprintf("/api/v1/articles/%d", deleteCandidate.ID), ``, "", agentB.Token)
	require.Equal(t, http.StatusNoContent, deleted.Code, deleted.Body.String())

	auditResponse := doJSONWithBearer(t, router, http.MethodGet, "/api/v1/audit-logs?limit=20", ``, "user-2", agentB.Token)
	require.Equal(t, http.StatusOK, auditResponse.Code, auditResponse.Body.String())
	var auditBody struct {
		Items []agentAuditResponse `json:"items"`
	}
	require.NoError(t, json.Unmarshal(auditResponse.Body.Bytes(), &auditBody))
	require.GreaterOrEqual(t, len(auditBody.Items), 5)
	assertAuditActionByAgent(t, auditBody.Items, "update_article", agentB.Agent.ID)
	assertAuditActionByAgent(t, auditBody.Items, "publish_article", agentB.Agent.ID)
	assertAuditActionByAgent(t, auditBody.Items, "delete_article", agentB.Agent.ID)
	for _, item := range auditBody.Items {
		require.Equal(t, "user-1", item.UserID)
	}
	for _, forbidden := range []string{"sensitive-body", "updated-sensitive-body", agentA.Token, agentB.Token, "access_token", "refresh_token", "image_url"} {
		require.NotContains(t, auditResponse.Body.String(), forbidden)
	}
	filteredAudit := doJSONWithBearer(t, router, http.MethodGet,
		"/api/v1/audit-logs?action=publish_article&agent_record_id="+agentB.Agent.ID, ``, "", agentA.Token)
	require.Equal(t, http.StatusOK, filteredAudit.Code)
	var filteredAuditBody struct {
		Items []agentAuditResponse `json:"items"`
	}
	require.NoError(t, json.Unmarshal(filteredAudit.Body.Bytes(), &filteredAuditBody))
	require.Len(t, filteredAuditBody.Items, 1)
	require.Equal(t, "publish_article", filteredAuditBody.Items[0].Action)

	otherUserAudit := doJSONWithBearer(t, router, http.MethodGet, "/api/v1/audit-logs", ``, "user-1", otherUserAgent.Token)
	require.Equal(t, http.StatusOK, otherUserAudit.Code)
	require.JSONEq(t, `{"items":[]}`, otherUserAudit.Body.String())
}

func assertAuditActionByAgent(t *testing.T, entries []agentAuditResponse, action string, agentRecordID string) {
	t.Helper()
	for _, entry := range entries {
		if entry.Action == action && entry.AgentRecordID == agentRecordID {
			return
		}
	}
	require.Failf(t, "audit action not found", "action %s for agent %s", action, agentRecordID)
}
