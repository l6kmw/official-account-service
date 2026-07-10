package agentmcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
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
		Name:        "official_account_create_article",
		Title:       "Create article",
		Description: "Create a local article draft. Publishing to WeChat is a separate explicit tool call.",
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
		Description: "Update a local article draft or revision, including title, digest, HTML body, and cover material binding.",
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
		Description: "Upload an article body image or cover image. Provide either file_path or content_base64. For cover images, the returned media_id must be attached to the article before publishing.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input uploadImageToolInput) (*mcp.CallToolResult, any, error) {
		content, filename, err := openUploadContent(input, cfg.AllowedRoot)
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
		Description: "Publish one local article to WeChat. Requires confirm_publish=true because this creates real public WeChat content.",
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
		Description: "Delete the WeChat article referenced by a publish record. Requires confirm_delete=\"DELETE\" because this removes real public WeChat content.",
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
		Description: "Return the public authorization entry URL for adding an official account. Use authorization_entry_url as the link, or qr_code_payload_url as the QR code payload.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input authorizationEntryToolInput) (*mcp.CallToolResult, any, error) {
		result, err := client.AuthorizationEntry(input.ComponentAppID)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"authorization_entry": result}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "official_account_generate_authorization_url",
		Title:       "Generate authorization URL",
		Description: "Generate a direct WeChat third-party-platform authorization URL for adding an official account. If component_appid or redirect_uri is omitted, the MCP environment defaults are used.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input authorizationURLToolInput) (*mcp.CallToolResult, any, error) {
		result, err := client.GenerateAuthorizationURL(ctx, input.ComponentAppID, input.RedirectURI, input.AuthType, input.BizAppID)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"authorization": result}, nil
	})

	return server
}

type listAccountsInput struct{}

type listArticlesInput struct{}

type listPublishRecordsInput struct{}

type createArticleToolInput struct {
	AuthorizerID int64  `json:"authorizer_id" jsonschema:"Authorized official account id returned by official_account_list_accounts."`
	Title        string `json:"title" jsonschema:"Article title."`
	Author       string `json:"author,omitempty" jsonschema:"Optional article author."`
	Digest       string `json:"digest,omitempty" jsonschema:"Optional article digest or summary."`
	ContentHTML  string `json:"content_html,omitempty" jsonschema:"WeChat-compatible article HTML body."`
}

type updateArticleToolInput struct {
	ArticleID         int64  `json:"article_id" jsonschema:"Local article id."`
	Title             string `json:"title" jsonschema:"Article title."`
	Author            string `json:"author,omitempty" jsonschema:"Optional article author."`
	Digest            string `json:"digest,omitempty" jsonschema:"Optional article digest or summary."`
	ContentHTML       string `json:"content_html,omitempty" jsonschema:"WeChat-compatible article HTML body."`
	CoverMediaAssetID int64  `json:"cover_media_asset_id,omitempty" jsonschema:"Material asset id of an uploaded cover image. Use 0 to keep no cover."`
}

type uploadImageToolInput struct {
	AuthorizerID  int64  `json:"authorizer_id" jsonschema:"Authorized official account id."`
	ArticleID     int64  `json:"article_id" jsonschema:"Local article id that owns this image."`
	Usage         string `json:"usage" jsonschema:"Image usage: inline_image for body images, or cover for cover images."`
	Filename      string `json:"filename,omitempty" jsonschema:"Filename sent to WeChat. Defaults to the file_path base name or image.png."`
	FilePath      string `json:"file_path,omitempty" jsonschema:"Local image file path readable by this MCP server. If OFFICIAL_ACCOUNT_MCP_ALLOWED_ROOT is set, the path must be inside it."`
	ContentBase64 string `json:"content_base64,omitempty" jsonschema:"Base64 encoded image content. Use this instead of file_path when the file is outside the allowed root."`
}

type publishArticleToolInput struct {
	ArticleID      int64 `json:"article_id" jsonschema:"Local article id to publish."`
	ConfirmPublish bool  `json:"confirm_publish" jsonschema:"Must be true to publish real WeChat content."`
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
	RedirectURI    string `json:"redirect_uri,omitempty" jsonschema:"Optional public HTTPS redirect URI configured in WeChat open platform. Defaults to the service authorization callback URL."`
	AuthType       int    `json:"auth_type,omitempty" jsonschema:"Optional WeChat authorization type: 1 official account, 2 mini program, 3 all."`
	BizAppID       string `json:"biz_appid,omitempty" jsonschema:"Optional authorizer appid to preselect."`
}

func openUploadContent(input uploadImageToolInput, allowedRoot string) (io.ReadCloser, string, error) {
	if input.Usage != "inline_image" && input.Usage != "cover" {
		return nil, "", fmt.Errorf("usage must be inline_image or cover")
	}
	hasFile := strings.TrimSpace(input.FilePath) != ""
	hasBase64 := strings.TrimSpace(input.ContentBase64) != ""
	if hasFile == hasBase64 {
		return nil, "", fmt.Errorf("provide exactly one of file_path or content_base64")
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
