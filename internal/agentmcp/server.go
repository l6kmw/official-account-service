package agentmcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ServerConfig contains MCP tool runtime guardrails.
type ServerConfig struct {
	AllowedRoot string
}

// NewServer creates a stdio-compatible MCP server for Official Account Service.
func NewServer(client *Client, cfg ServerConfig) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "official-account-service",
		Title:   "Official Account Service",
		Version: "0.1.0",
	}, nil)
	server.AddReceivingMiddleware(apiTokenContextMiddleware, toolGuidanceMiddleware)
	remoteImageHTTPClient := newRemoteImageHTTPClient()

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_list_accounts",
		Title:       "List official accounts",
		Description: "List authorized WeChat official accounts for the configured tenant. Does not expose token, secret, or refresh-token fields.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ listAccountsInput) (*mcp.CallToolResult, any, error) {
		items, err := client.ListAccounts(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"items": items}, nil
	})

	registerLocalDraftTools(server, client)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_list_articles",
		Title:       "List articles",
		Description: "List local article drafts and published articles for the configured tenant.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ listArticlesInput) (*mcp.CallToolResult, any, error) {
		items, err := client.ListArticles(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"items": items}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_list_published_articles",
		Title:       "List live WeChat published articles",
		Description: "Read the current published article list directly from WeChat, including articles published outside this service. Required: authorizer_id. Pagination offset/count applies to WeChat messages; one message may contain multiple returned articles. count must be 1-20. Body HTML and deleted entries are omitted by default. When available, each article includes msgid for immediate use with official_account_list_article_comments.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input listPublishedArticlesToolInput) (*mcp.CallToolResult, any, error) {
		count := input.Count
		if count == 0 {
			count = 20
		}
		result, err := client.ListPublishedArticles(ctx, input.AuthorizerID, input.Offset, count, input.IncludeContent, input.IncludeDeleted)
		if err != nil {
			return nil, nil, err
		}
		return nil, result, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_list_permanent_materials",
		Title:       "List WeChat permanent materials",
		Description: "List permanent image materials directly from one authorized WeChat account. Required: authorizer_id. offset defaults to 0 and count defaults to 20; count must be 1-20. Returns media_id, name, update_time, and URL without exposing credentials.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input listPermanentMaterialsToolInput) (*mcp.CallToolResult, any, error) {
		count := input.Count
		if count == 0 {
			count = 20
		}
		result, err := client.ListPermanentMaterials(ctx, input.AuthorizerID, input.Offset, count)
		if err != nil {
			return nil, nil, err
		}
		return nil, result, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_delete_permanent_material",
		Title:       "Delete WeChat permanent material",
		Description: "Permanently delete one material from an authorized WeChat account. Required: authorizer_id, media_id, and confirm_delete=\"DELETE\". This is irreversible and may break drafts or articles that still reference the media_id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input deletePermanentMaterialToolInput) (*mcp.CallToolResult, any, error) {
		if input.ConfirmDelete != "DELETE" {
			return nil, nil, fmt.Errorf("confirm_delete must be DELETE before permanently deleting WeChat material")
		}
		if err := client.DeletePermanentMaterial(ctx, input.AuthorizerID, input.MediaID); err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"deleted": true, "media_id": input.MediaID}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_get_article_metrics",
		Title:       "Get WeChat article metrics",
		Description: "Get live cumulative metrics for all articles published on one date, including reads, shares, likes, comments, collections, rewards, conversions, completion rate, sources, and drop-off positions. Required: authorizer_id and date (YYYY-MM-DD). WeChat supports one publish date per call, from 2025-11-01 through yesterday, and metrics are limited to the first 30 days after publication.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input getArticleMetricsToolInput) (*mcp.CallToolResult, any, error) {
		result, err := client.GetArticleMetrics(ctx, input.AuthorizerID, input.Date)
		if err != nil {
			return nil, nil, err
		}
		return nil, result, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_list_article_comments",
		Title:       "List WeChat article comments",
		Description: "List live comments for one published WeChat article. Required: authorizer_id and msgid returned by official_account_list_published_articles or official_account_get_article_metrics. Optional begin, count (1-49, default 20), and type (0 all, 1 ordinary, 2 selected). User OpenIDs are never returned.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input listArticleCommentsToolInput) (*mcp.CallToolResult, any, error) {
		count := input.Count
		if count == 0 {
			count = 20
		}
		result, err := client.ListArticleComments(ctx, input.AuthorizerID, input.MsgID, input.Begin, count, input.Type)
		if err != nil {
			return nil, nil, err
		}
		return nil, result, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_create_article",
		Title:       "Create article",
		Description: "Create a local article draft. Required: authorizer_id and title. Optional: author, digest, and content_html. Publishing to WeChat is a separate explicit tool call and additionally requires non-empty content_html plus an uploaded cover bound through cover_media_asset_id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input createArticleToolInput) (*mcp.CallToolResult, any, error) {
		article, err := client.CreateArticle(ctx, CreateArticleInput{
			AuthorizerID: input.AuthorizerID,
			Title:        input.Title,
			Author:       input.Author,
			Digest:       input.Digest,
			ContentHTML:  input.ContentHTML,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"article": article}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_update_article",
		Title:       "Update article",
		Description: "Replace the editable fields of a local article draft or revision. Required in every update: article_id, title, author, digest, content_html, and cover_media_asset_id. author and digest may be empty strings, and cover_media_asset_id may be 0 while drafting. Publishing later requires non-empty content_html and a cover asset id returned by official_account_upload_image.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input updateArticleToolInput) (*mcp.CallToolResult, any, error) {
		article, err := client.UpdateArticle(ctx, input.ArticleID, UpdateArticleInput{
			Title:             input.Title,
			Author:            input.Author,
			Digest:            input.Digest,
			ContentHTML:       input.ContentHTML,
			CoverMediaAssetID: input.CoverMediaAssetID,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"article": article}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_upload_image",
		Title:       "Upload image",
		Description: "Upload an article body image or cover image. Required: authorizer_id, article_id, usage, and exactly one of file_path, content_base64, or image_url. image_url downloads a public HTTPS image for online agents. filename is optional. For a cover, pass the returned asset.id as cover_media_asset_id to official_account_update_article before publishing; do not use media_id for that field.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input uploadImageToolInput) (*mcp.CallToolResult, any, error) {
		content, filename, err := openUploadContent(ctx, input, cfg.AllowedRoot, remoteImageHTTPClient)
		if err != nil {
			return nil, nil, err
		}
		defer content.Close()
		asset, err := client.UploadImage(ctx, UploadImageInput{
			AuthorizerID: input.AuthorizerID,
			ArticleID:    input.ArticleID,
			Usage:        input.Usage,
			Filename:     filename,
			Content:      content,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"asset": asset}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_publish_article",
		Title:       "Publish article",
		Description: "Publish one local article to WeChat. Required tool inputs: article_id and confirm_publish=true. The article must already have a non-empty title, non-empty content_html, and a valid cover_media_asset_id because this creates real public WeChat content.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input publishArticleToolInput) (*mcp.CallToolResult, any, error) {
		if !input.ConfirmPublish {
			return nil, nil, fmt.Errorf("confirm_publish must be true before publishing real WeChat content")
		}
		record, err := client.PublishArticle(ctx, input.ArticleID)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"record": record}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_delete_article",
		Title:       "Delete article completely",
		Description: "Delete one article completely. The backend removes all published WeChat copies before deleting the local article and its materials; deleted publish records remain for audit. Requires confirm_delete=\"DELETE\".",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input deleteArticleToolInput) (*mcp.CallToolResult, any, error) {
		if input.ConfirmDelete != "DELETE" {
			return nil, nil, fmt.Errorf("confirm_delete must be DELETE before deleting an article and its published WeChat content")
		}
		article, err := client.DeleteArticle(ctx, input.ArticleID)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"deleted_article": article}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_list_publish_records",
		Title:       "List publish records",
		Description: "List WeChat publish records for the configured tenant.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ listPublishRecordsInput) (*mcp.CallToolResult, any, error) {
		items, err := client.ListPublishRecords(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"items": items}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_sync_publish_status",
		Title:       "Sync publish status",
		Description: "Poll WeChat for one publish record status and update local state.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input syncPublishStatusToolInput) (*mcp.CallToolResult, any, error) {
		record, err := client.SyncPublishStatus(ctx, input.PublishRecordID)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"record": record}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_delete_published_record",
		Title:       "Delete published WeChat content",
		Description: "Delete only the WeChat article referenced by one publish record while keeping the local article. Requires confirm_delete=\"DELETE\" because this removes real public WeChat content.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input deletePublishedToolInput) (*mcp.CallToolResult, any, error) {
		if input.ConfirmDelete != "DELETE" {
			return nil, nil, fmt.Errorf("confirm_delete must be DELETE before deleting real WeChat content")
		}
		record, err := client.DeletePublishedRecord(ctx, input.PublishRecordID)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"record": record}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_get_authorization_entry",
		Title:       "Get authorization entry",
		Description: "Generate a first-party authorization-entry URL bound to the authenticated MCP user. Open authorization_entry_url directly, or render qr_code_payload_url as a QR code; the page validates the target and then redirects to WeChat.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input authorizationEntryToolInput) (*mcp.CallToolResult, any, error) {
		result, err := client.GenerateAuthorizationURL(ctx, input.ComponentAppID, "", 0, "")
		if err != nil {
			return nil, nil, err
		}
		entry, err := client.AuthorizationEntryForURL(input.ComponentAppID, result.AuthorizationURL)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"authorization_entry": entry}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_generate_authorization_url",
		Title:       "Generate advanced authorization entry",
		Description: "Generate a first-party authorization-entry URL with optional auth_type or biz_appid. Open authorization_url directly; it validates the user-bound WeChat target before redirecting to the QR page. Omit redirect_uri unless it exactly matches the configured service callback.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input authorizationURLToolInput) (*mcp.CallToolResult, any, error) {
		redirectURI := strings.TrimSpace(input.RedirectURI)
		if redirectURI != "" && redirectURI != client.authorizationCallbackURL() {
			return nil, nil, fmt.Errorf("redirect_uri must match the configured service callback")
		}
		result, err := client.GenerateAuthorizationURL(ctx, input.ComponentAppID, redirectURI, input.AuthType, input.BizAppID)
		if err != nil {
			return nil, nil, err
		}
		entry, err := client.AuthorizationEntryForURL(input.ComponentAppID, result.AuthorizationURL)
		if err != nil {
			return nil, nil, err
		}
		result.AuthorizationURL = entry.AuthorizationEntryURL
		return nil, map[string]any{"authorization": result}, nil
	})

	return server
}

func registerLocalDraftTools(server *mcp.Server, client *Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_list_drafts",
		Title:       "List local drafts",
		Description: "List unpublished drafts stored in this service for the authenticated MCP user. Includes drafts whose previous publish attempt failed; excludes publishing and published articles.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ listArticlesInput) (*mcp.CallToolResult, any, error) {
		articles, err := client.ListArticles(ctx)
		if err != nil {
			return nil, nil, err
		}
		drafts := make([]Article, 0, len(articles))
		for _, item := range articles {
			if isEditableLocalDraft(item) {
				drafts = append(drafts, item)
			}
		}
		return nil, map[string]any{"items": drafts}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_get_draft",
		Title:       "Get local draft",
		Description: "Get the full local draft, including content_html and cover_media_asset_id. Required: draft_id returned by official_account_list_drafts or official_account_create_draft.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input getDraftToolInput) (*mcp.CallToolResult, any, error) {
		draft, err := getEditableLocalDraft(ctx, client, input.DraftID)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"draft": draft}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_create_draft",
		Title:       "Create local draft",
		Description: "Create a draft in this service's local database. Required: authorizer_id and title. Optional: author, digest, and content_html. This does not create or publish content on WeChat.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input createArticleToolInput) (*mcp.CallToolResult, any, error) {
		draft, err := client.CreateArticle(ctx, CreateArticleInput{
			AuthorizerID: input.AuthorizerID,
			Title:        input.Title,
			Author:       input.Author,
			Digest:       input.Digest,
			ContentHTML:  input.ContentHTML,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"draft": draft}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_update_draft",
		Title:       "Update local draft",
		Description: "Replace an editable local draft. Required in every update: draft_id, title, author, digest, content_html, and cover_media_asset_id. author and digest may be empty, and cover_media_asset_id may be 0 while drafting.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input updateDraftToolInput) (*mcp.CallToolResult, any, error) {
		if _, err := getEditableLocalDraft(ctx, client, input.DraftID); err != nil {
			return nil, nil, err
		}
		draft, err := client.UpdateArticle(ctx, input.DraftID, UpdateArticleInput{
			Title:             input.Title,
			Author:            input.Author,
			Digest:            input.Digest,
			ContentHTML:       input.ContentHTML,
			CoverMediaAssetID: input.CoverMediaAssetID,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"draft": draft}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_delete_draft",
		Title:       "Delete local draft",
		Description: "Delete one unpublished local draft and its local materials. Required: draft_id and confirm_delete=\"DELETE\". This tool refuses publishing or published articles.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input deleteDraftToolInput) (*mcp.CallToolResult, any, error) {
		if input.ConfirmDelete != "DELETE" {
			return nil, nil, fmt.Errorf("confirm_delete must be DELETE before deleting a local draft")
		}
		if _, err := getEditableLocalDraft(ctx, client, input.DraftID); err != nil {
			return nil, nil, err
		}
		draft, err := client.DeleteArticle(ctx, input.DraftID)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"deleted_draft": draft}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_publish_draft",
		Title:       "Publish local draft",
		Description: "Submit one local draft through this service's WeChat publish pipeline. Required: draft_id and confirm_publish=true. The local draft must have content_html and a valid uploaded cover bound through cover_media_asset_id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input publishDraftToolInput) (*mcp.CallToolResult, any, error) {
		if !input.ConfirmPublish {
			return nil, nil, fmt.Errorf("confirm_publish must be true before publishing a local draft to WeChat")
		}
		if _, err := getEditableLocalDraft(ctx, client, input.DraftID); err != nil {
			return nil, nil, err
		}
		record, err := client.PublishArticle(ctx, input.DraftID)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"record": record}, nil
	})
}

func getEditableLocalDraft(ctx context.Context, client *Client, draftID int64) (Article, error) {
	draft, err := client.GetArticle(ctx, draftID)
	if err != nil {
		return Article{}, err
	}
	if !isEditableLocalDraft(draft) {
		return Article{}, fmt.Errorf("article %d is not an editable local draft (status %q)", draftID, draft.Status)
	}
	return draft, nil
}

func isEditableLocalDraft(item Article) bool {
	return item.Status == "draft" || item.Status == "failed"
}

func apiTokenContextMiddleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		extra := req.GetExtra()
		if extra != nil && extra.TokenInfo != nil && extra.TokenInfo.Extra != nil && extra.TokenInfo.Extra[CredentialKindExtraKey] == UserAPICredentialKind {
			if token := bearerTokenFromHeader(extra.Header.Get("Authorization")); token != "" {
				ctx = context.WithValue(ctx, apiTokenContextKey{}, token)
			}
		}
		return next(ctx, method, req)
	}
}

func bearerTokenFromHeader(header string) string {
	const prefix = "Bearer "
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}

type listAccountsInput struct{}

type listArticlesInput struct{}

type getDraftToolInput struct {
	DraftID int64 `json:"draft_id" jsonschema:"Required. Local draft id returned by official_account_list_drafts or official_account_create_draft."`
}

type listPublishedArticlesToolInput struct {
	AuthorizerID   int64 `json:"authorizer_id" jsonschema:"Required. Authorized official account id returned by official_account_list_accounts."`
	Offset         int   `json:"offset,omitempty" jsonschema:"WeChat message offset. Defaults to 0."`
	Count          int   `json:"count,omitempty" jsonschema:"Number of WeChat messages to fetch, 1-20. Defaults to 20."`
	IncludeContent bool  `json:"include_content,omitempty" jsonschema:"Include full article HTML. Defaults to false to keep responses small."`
	IncludeDeleted bool  `json:"include_deleted,omitempty" jsonschema:"Include entries marked deleted by WeChat. Defaults to false."`
}

type getArticleMetricsToolInput struct {
	AuthorizerID int64  `json:"authorizer_id" jsonschema:"Required. Authorized official account id."`
	Date         string `json:"date" jsonschema:"Required. Article publication date in YYYY-MM-DD format. Must be between 2025-11-01 and yesterday."`
}

type listPermanentMaterialsToolInput struct {
	AuthorizerID int64 `json:"authorizer_id" jsonschema:"Required. Authorized official account id returned by official_account_list_accounts."`
	Offset       int   `json:"offset,omitempty" jsonschema:"Permanent image offset. Defaults to 0."`
	Count        int   `json:"count,omitempty" jsonschema:"Number of permanent images to fetch, 1-20. Defaults to 20."`
}

type deletePermanentMaterialToolInput struct {
	AuthorizerID  int64  `json:"authorizer_id" jsonschema:"Required. Authorized official account id returned by official_account_list_accounts."`
	MediaID       string `json:"media_id" jsonschema:"Required. WeChat permanent material media_id returned by official_account_list_permanent_materials or official_account_upload_image."`
	ConfirmDelete string `json:"confirm_delete" jsonschema:"Required. Must be exactly DELETE because permanent material deletion is irreversible."`
}

type listArticleCommentsToolInput struct {
	AuthorizerID int64  `json:"authorizer_id" jsonschema:"Required. Authorized official account id."`
	MsgID        string `json:"msgid" jsonschema:"Required. Article msgid returned by official_account_list_published_articles or official_account_get_article_metrics, for example 2247490098_1."`
	Begin        int    `json:"begin,omitempty" jsonschema:"Comment offset. Defaults to 0."`
	Count        int    `json:"count,omitempty" jsonschema:"Number of comments to fetch, 1-49. Defaults to 20."`
	Type         int    `json:"type,omitempty" jsonschema:"Comment filter: 0 all, 1 ordinary, 2 selected. Defaults to 0."`
}

type listPublishRecordsInput struct{}

type createArticleToolInput struct {
	AuthorizerID int64  `json:"authorizer_id" jsonschema:"Required. Authorized official account id returned by official_account_list_accounts."`
	Title        string `json:"title" jsonschema:"Required. Non-empty article title."`
	Author       string `json:"author,omitempty" jsonschema:"Optional article author."`
	Digest       string `json:"digest,omitempty" jsonschema:"Optional article digest or summary."`
	ContentHTML  string `json:"content_html,omitempty" jsonschema:"Optional while creating a draft, but required and non-empty before publishing. WeChat-compatible article HTML body."`
}

type updateArticleToolInput struct {
	ArticleID         int64  `json:"article_id" jsonschema:"Required. Local article id."`
	Title             string `json:"title" jsonschema:"Required. Non-empty article title."`
	Author            string `json:"author" jsonschema:"Required in the update request to avoid overwriting an existing value by omission. May be an empty string."`
	Digest            string `json:"digest" jsonschema:"Required in the update request to avoid overwriting an existing value by omission. May be an empty string."`
	ContentHTML       string `json:"content_html" jsonschema:"Required in the update request. May be empty while drafting, but must be non-empty before publishing. WeChat-compatible article HTML body."`
	CoverMediaAssetID int64  `json:"cover_media_asset_id" jsonschema:"Required in the update request. Use the asset.id returned by official_account_upload_image with usage=cover; use 0 to keep no cover while drafting."`
}

type updateDraftToolInput struct {
	DraftID           int64  `json:"draft_id" jsonschema:"Required. Local draft id returned by official_account_list_drafts or official_account_create_draft."`
	Title             string `json:"title" jsonschema:"Required in the update request. Full replacement title."`
	Author            string `json:"author" jsonschema:"Required in the update request. May be an empty string."`
	Digest            string `json:"digest" jsonschema:"Required in the update request. May be an empty string."`
	ContentHTML       string `json:"content_html" jsonschema:"Required in the update request. May be empty while drafting, but publishing requires non-empty HTML."`
	CoverMediaAssetID int64  `json:"cover_media_asset_id" jsonschema:"Required in the update request. Use the asset.id returned by official_account_upload_image with usage=cover; use 0 to keep no cover while drafting."`
}

type uploadImageToolInput struct {
	AuthorizerID  int64  `json:"authorizer_id" jsonschema:"Required. Authorized official account id."`
	ArticleID     int64  `json:"article_id" jsonschema:"Required. Local article id that owns this image."`
	Usage         string `json:"usage" jsonschema:"Required. Image usage: inline_image for body images, or cover for cover images."`
	Filename      string `json:"filename,omitempty" jsonschema:"Filename sent to WeChat. Defaults to the source filename or a name inferred from the image type."`
	FilePath      string `json:"file_path,omitempty" jsonschema:"Conditionally required: provide exactly one of file_path, content_base64, or image_url. Local image file path readable by this MCP server. If OFFICIAL_ACCOUNT_MCP_ALLOWED_ROOT is set, the path must be inside it."`
	ContentBase64 string `json:"content_base64,omitempty" jsonschema:"Conditionally required: provide exactly one of content_base64, file_path, or image_url. Base64 encoded image content."`
	ImageURL      string `json:"image_url,omitempty" jsonschema:"Conditionally required: provide exactly one of image_url, file_path, or content_base64. Public HTTPS URL for an image uploaded to the online agent. Private and local network destinations are rejected."`
}

type publishArticleToolInput struct {
	ArticleID      int64 `json:"article_id" jsonschema:"Required. Local article id to publish."`
	ConfirmPublish bool  `json:"confirm_publish" jsonschema:"Required and must be true to publish real WeChat content."`
}

type publishDraftToolInput struct {
	DraftID        int64 `json:"draft_id" jsonschema:"Required. Editable local draft id returned by official_account_list_drafts or official_account_create_draft."`
	ConfirmPublish bool  `json:"confirm_publish" jsonschema:"Required. Must be true after the user confirms publishing real WeChat content."`
}

type deleteArticleToolInput struct {
	ArticleID     int64  `json:"article_id" jsonschema:"Local article id to delete."`
	ConfirmDelete string `json:"confirm_delete" jsonschema:"Must be DELETE to delete the local article."`
}

type deleteDraftToolInput struct {
	DraftID       int64  `json:"draft_id" jsonschema:"Required. Editable local draft id returned by official_account_list_drafts or official_account_create_draft."`
	ConfirmDelete string `json:"confirm_delete" jsonschema:"Required. Must equal DELETE after explicit confirmation."`
}

type syncPublishStatusToolInput struct {
	PublishRecordID int64 `json:"publish_record_id" jsonschema:"Local publish record id."`
}

type deletePublishedToolInput struct {
	PublishRecordID int64  `json:"publish_record_id" jsonschema:"Local publish record id whose WeChat article should be deleted."`
	ConfirmDelete   string `json:"confirm_delete" jsonschema:"Must be DELETE to remove real WeChat content."`
}

type authorizationEntryToolInput struct {
	ComponentAppID string `json:"component_appid,omitempty" jsonschema:"Optional WeChat third-party-platform component appid. Defaults to OFFICIAL_ACCOUNT_COMPONENT_APP_ID."`
}

type authorizationURLToolInput struct {
	ComponentAppID string `json:"component_appid,omitempty" jsonschema:"Optional WeChat third-party-platform component appid. Defaults to OFFICIAL_ACCOUNT_COMPONENT_APP_ID."`
	RedirectURI    string `json:"redirect_uri,omitempty" jsonschema:"Optional configured service authorization callback URL. Omit this unless it exactly matches the MCP service callback."`
	AuthType       int    `json:"auth_type,omitempty" jsonschema:"Optional WeChat authorization type: 1 official account, 2 mini program, 3 all."`
	BizAppID       string `json:"biz_appid,omitempty" jsonschema:"Optional authorizer appid to preselect."`
}

func openUploadContent(ctx context.Context, input uploadImageToolInput, allowedRoot string, remoteImageHTTPClient *http.Client) (io.ReadCloser, string, error) {
	if input.Usage != "inline_image" && input.Usage != "cover" {
		return nil, "", fmt.Errorf("usage must be inline_image or cover")
	}
	hasFile := strings.TrimSpace(input.FilePath) != ""
	hasBase64 := strings.TrimSpace(input.ContentBase64) != ""
	hasImageURL := strings.TrimSpace(input.ImageURL) != ""
	sourceCount := 0
	for _, present := range []bool{hasFile, hasBase64, hasImageURL} {
		if present {
			sourceCount++
		}
	}
	if sourceCount != 1 {
		return nil, "", fmt.Errorf("provide exactly one of file_path, content_base64, or image_url")
	}
	filename := strings.TrimSpace(input.Filename)
	if hasBase64 {
		raw, err := base64.StdEncoding.DecodeString(input.ContentBase64)
		if err != nil {
			return nil, "", fmt.Errorf("decode content_base64: %w", err)
		}
		if filename == "" {
			filename = "image.png"
		}
		return io.NopCloser(bytes.NewReader(raw)), filename, nil
	}
	if hasImageURL {
		return downloadRemoteImage(ctx, remoteImageHTTPClient, input.ImageURL, filename)
	}

	path, err := allowedFilePath(input.FilePath, allowedRoot)
	if err != nil {
		return nil, "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("open upload image: %w", err)
	}
	if filename == "" {
		filename = filepath.Base(path)
	}
	return file, filename, nil
}

func allowedFilePath(rawPath string, allowedRoot string) (string, error) {
	path, err := filepath.Abs(strings.TrimSpace(rawPath))
	if err != nil {
		return "", fmt.Errorf("resolve file_path: %w", err)
	}
	allowedRoot = strings.TrimSpace(allowedRoot)
	if allowedRoot == "" {
		return path, nil
	}
	root, err := filepath.Abs(allowedRoot)
	if err != nil {
		return "", fmt.Errorf("resolve allowed root: %w", err)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("check allowed root: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("file_path must be inside OFFICIAL_ACCOUNT_MCP_ALLOWED_ROOT")
	}
	return path, nil
}
