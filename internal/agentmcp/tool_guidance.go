package agentmcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type toolFieldRule struct {
	Required  bool
	Expected  string
	Fix       string
	Sensitive bool
	Validate  func(any) bool
}

type toolGuide struct {
	Before        string
	Inputs        string
	Returns       string
	Next          string
	Check         string
	Fields        map[string]toolFieldRule
	ValidateGroup func(map[string]any) []toolInputIssue
}

type toolInputIssue struct {
	Code     string `json:"code"`
	Field    string `json:"field"`
	Received string `json:"received"`
	Expected string `json:"expected"`
	Fix      string `json:"fix"`
}

var officialAccountToolGuides = buildOfficialAccountToolGuides()

func buildOfficialAccountToolGuides() map[string]toolGuide {
	return map[string]toolGuide{
		"official_account_get_identity": {
			Before:  "我将确认当前 MCP Token 对应的用户和 Agent 身份，不会返回 Token、Token 提示或 Token 摘要。",
			Inputs:  "无。身份完全由当前 MCP Token 决定，不能通过参数指定或伪造 Agent。",
			Returns: "identity；包含 username、user_id、role、actor_type、agent_record_id、agent_id、agent_name 和 agent_purpose。",
			Next:    "确认 Agent 身份和内容职责后，再调用 list_accounts 选择该用户已授权的公众号。",
			Check:   "确认 MCP Token 有效、所属用户和 Agent 均为 active，且 Token 未被轮换或撤销。",
			Fields:  noToolFields(),
		},
		"official_account_list_accounts": {
			Before:  "我将读取你已授权的公众号列表，不会返回 AppSecret、access token 或 refresh token。",
			Inputs:  "无。账号范围由当前 MCP Token 自动确定。",
			Returns: "items；后续工具使用 items[].id 作为 authorizer_id。",
			Next:    "没有账号时调用 official_account_get_authorization_entry；有账号时先让用户确认要操作的公众号。",
			Check:   "确认 MCP Token 有效且当前用户未被停用。",
			Fields:  noToolFields(),
		},
		"official_account_list_drafts": {
			Before:  "我将读取当前用户的本地未发布草稿和可重试的失败草稿，不读取微信草稿箱。",
			Inputs:  "无。",
			Returns: "items，包含 draft/failed 草稿及其本地 draft id。",
			Next:    "使用 draft id 调用 get_draft、update_draft、publish_draft 或 delete_draft。",
			Check:   "若结果为空，先创建草稿或确认使用了正确的用户 Token。",
			Fields:  noToolFields(),
		},
		"official_account_get_draft": {
			Before:  "我将读取指定本地草稿的完整正文、封面关联和状态。",
			Inputs:  "必填 draft_id，来自 list_drafts 或 create_draft。",
			Returns: "draft；包含 content_html、cover_media_asset_id 和 status。",
			Next:    "编辑时调用 update_draft；发布前确认正文和封面后调用 publish_draft。",
			Check:   "先调用 official_account_list_drafts，确认 draft_id 属于当前用户，且状态为 draft 或 failed。",
			Fields: map[string]toolFieldRule{
				"draft_id": requiredPositiveID("先调用 official_account_list_drafts，并使用返回的 id。"),
			},
		},
		"official_account_create_draft": {
			Before:  "我将为你选择的公众号创建本地草稿；这一步不会写入微信草稿箱，也不会公开发布。",
			Inputs:  "必填 authorizer_id、title；author、digest、content_html 可选。",
			Returns: "draft；保存 draft.id，后续上传图片、编辑和发布都要使用它。",
			Next:    "如需图片先调用 upload_image，再用 update_draft 写入正文和 cover_media_asset_id。",
			Check:   "确认 authorizer_id 来自 list_accounts，title 非空。",
			Fields:  createContentFields(),
		},
		"official_account_update_draft": {
			Before:  "我将完整替换这个本地草稿的可编辑字段；未传内容不会自动保留旧值。",
			Inputs:  "必填 draft_id、title、author、digest、content_html、cover_media_asset_id；author/digest 可为空，草稿阶段封面可为 0。",
			Returns: "draft；返回保存后的完整草稿。",
			Next:    "继续编辑，或确认公众号、标题、正文和封面后调用 publish_draft。",
			Check:   "确认草稿仍为 draft/failed；封面字段使用 upload_image 返回的 asset.id，不是 media_id。",
			Fields:  updateContentFields("draft_id", "先调用 official_account_list_drafts，并使用返回的 id。"),
		},
		"official_account_delete_draft": {
			Before:  "我将永久删除这个未发布本地草稿及本地素材；发布中或已发布文章不会被本工具删除。",
			Inputs:  "必填 draft_id、confirm_delete；confirm_delete 必须精确填写 DELETE。",
			Returns: "deleted_draft；表示本地草稿已删除。",
			Next:    "调用 list_drafts 确认草稿不再存在。",
			Check:   "先让用户确认删除；确认草稿状态为 draft/failed。",
			Fields: map[string]toolFieldRule{
				"draft_id":       requiredPositiveID("先调用 official_account_list_drafts，并使用返回的 id。"),
				"confirm_delete": requiredExactString("DELETE", "用户明确确认后填写 DELETE；不要替用户自动确认。"),
			},
		},
		"official_account_publish_draft": {
			Before:  "我将把这个本地草稿提交到微信并真实公开发布；请先确认公众号、标题、摘要、正文和封面。",
			Inputs:  "必填 draft_id、confirm_publish=true；草稿必须有非空正文和有效封面 asset.id。",
			Returns: "record；保存 record.id，并用 sync_publish_status 同步到 published/failed。",
			Next:    "调用 sync_publish_status，之后用 list_published_articles 核对微信实时列表。",
			Check:   "若发布前校验失败，先 get_draft，再补正文或通过 upload_image/update_draft 设置封面。",
			Fields: map[string]toolFieldRule{
				"draft_id":        requiredPositiveID("先调用 official_account_list_drafts，并使用返回的 id。"),
				"confirm_publish": requiredTrue("用户明确确认真实发布后填写 true；不要替用户自动确认。"),
			},
		},
		"official_account_list_articles": {
			Before:  "我将读取当前用户全部本地文章，包含草稿、发布中、已发布和失败状态。",
			Inputs:  "无。",
			Returns: "items；每条包含本地 article id、authorizer_id 和 status。",
			Next:    "使用 article id 调用兼容的 update_article、publish_article 或 delete_article。",
			Check:   "若需要微信实时文章而不是本地记录，请改用 list_published_articles。",
			Fields:  noToolFields(),
		},
		"official_account_list_published_articles": {
			Before:  "我将直接读取指定公众号当前的微信已发布文章列表，包括不经本系统发布的历史文章。",
			Inputs:  "必填 authorizer_id；offset>=0；count 省略或为 1-20；include_content/include_deleted 可选。",
			Returns: "items、分页信息；items[].msgid 可直接查询评论。",
			Next:    "需要评论时使用 msgid 调用 list_article_comments；需要数据时按发表日期调用 get_article_metrics。",
			Check:   "确认公众号已授权发布权限；分页参数针对微信消息，一条消息可能含多篇文章。",
			Fields: map[string]toolFieldRule{
				"authorizer_id":   requiredAuthorizerID(),
				"offset":          optionalNonNegativeInteger("省略或填写大于等于 0 的微信消息偏移量。"),
				"count":           optionalDefaultedRange(1, 20, "省略使用 20，或填写 1-20。"),
				"include_content": optionalBoolean("填写 true 返回正文 HTML；省略为 false。"),
				"include_deleted": optionalBoolean("填写 true 包含微信标记删除的条目；省略为 false。"),
			},
		},
		"official_account_list_permanent_materials": {
			Before:  "我将直接读取指定公众号的微信永久图片素材库，不会返回 access token。",
			Inputs:  "必填 authorizer_id；offset>=0；count 省略或为 1-20。",
			Returns: "items、分页信息；items[].media_id 用于明确选择和删除素材。",
			Next:    "删除前向用户展示素材名称和 media_id，再调用 delete_permanent_material。",
			Check:   "微信永久素材和本地文章素材是两套列表；不要凭本地 asset.id 猜 media_id。",
			Fields: map[string]toolFieldRule{
				"authorizer_id": requiredAuthorizerID(),
				"offset":        optionalNonNegativeInteger("省略或填写大于等于 0 的素材偏移量。"),
				"count":         optionalDefaultedRange(1, 20, "省略使用 20，或填写 1-20。"),
			},
		},
		"official_account_delete_permanent_material": {
			Before:  "我将不可恢复地删除指定微信永久素材；仍引用它的草稿或文章可能出现图片失效。",
			Inputs:  "必填 authorizer_id、media_id、confirm_delete=DELETE。",
			Returns: "deleted=true 和被删除的 media_id。",
			Next:    "调用 list_permanent_materials 确认素材已消失，并检查引用它的草稿。",
			Check:   "先用 list_permanent_materials 核对名称和 media_id，并取得用户明确确认。",
			Fields: map[string]toolFieldRule{
				"authorizer_id":  requiredAuthorizerID(),
				"media_id":       requiredNonEmptyString("先调用 official_account_list_permanent_materials，并原样使用返回的 media_id。", false),
				"confirm_delete": requiredExactString("DELETE", "用户明确确认不可恢复删除后填写 DELETE。"),
			},
		},
		"official_account_get_article_metrics": {
			Before:  "我将查询指定公众号某一天发布文章的微信累计阅读、分享、点赞、评论、收藏和完成率数据。",
			Inputs:  "必填 authorizer_id、date；date 格式 YYYY-MM-DD，范围为 2025-11-01 到昨天。",
			Returns: "date、delayed、items；items 可能为空，数据只保留文章发表后 30 天。",
			Next:    "按 items[].msgid 查询评论，或向用户汇总各文章指标。",
			Check:   "不能查询今天；确认公众号具备数据统计权限且日期在允许范围内。",
			Fields: map[string]toolFieldRule{
				"authorizer_id": requiredAuthorizerID(),
				"date":          requiredMetricsDate(),
			},
		},
		"official_account_list_article_comments": {
			Before:  "我将读取指定微信已发布文章的实时评论，不会返回评论者 OpenID。",
			Inputs:  "必填 authorizer_id、msgid；begin>=0；count 省略或为 1-49；type 为 0全部/1普通/2精选。",
			Returns: "items、total、returned_count、next_begin 和 has_more。",
			Next:    "has_more=true 时用 next_begin 继续分页。",
			Check:   "msgid 必须来自 list_published_articles 或 get_article_metrics，不要使用本地 article id。",
			Fields: map[string]toolFieldRule{
				"authorizer_id": requiredAuthorizerID(),
				"msgid":         requiredNonEmptyString("先调用 official_account_list_published_articles，并使用返回的 msgid。", false),
				"begin":         optionalNonNegativeInteger("省略或填写大于等于 0 的评论偏移量。"),
				"count":         optionalDefaultedRange(1, 49, "省略使用 20，或填写 1-49。"),
				"type":          optionalIntegerEnum([]int64{0, 1, 2}, "填写 0（全部）、1（普通）或 2（精选）。"),
			},
		},
		"official_account_create_article": {
			Before:  "我将通过兼容接口创建本地文章草稿；这一步不会真实发布。新 Agent 优先使用 create_draft。",
			Inputs:  "必填 authorizer_id、title；author、digest、content_html 可选。",
			Returns: "article；保存 article.id 用于兼容的编辑、上传和发布工具。",
			Next:    "上传图片后调用 update_article，确认内容后调用 publish_article。",
			Check:   "确认 authorizer_id 来自 list_accounts，title 非空。",
			Fields:  createContentFields(),
		},
		"official_account_update_article": {
			Before:  "我将通过兼容接口完整替换本地文章的可编辑字段；未传内容不会自动保留旧值。",
			Inputs:  "必填 article_id、title、author、digest、content_html、cover_media_asset_id。",
			Returns: "article；返回保存后的完整本地文章。",
			Next:    "确认公众号、标题、正文和封面后调用 publish_article。",
			Check:   "封面字段必须使用 upload_image 返回的 asset.id，不是微信 media_id。",
			Fields:  updateContentFields("article_id", "先调用 official_account_list_articles，并使用返回的 id。"),
		},
		"official_account_upload_image": {
			Before:  "我将把图片上传到指定文章；正文图返回 wechat_url，封面图返回可绑定的 asset.id 和微信 media_id。",
			Inputs:  "必填 authorizer_id、article_id、usage；usage 为 inline_image 或 cover；file_path/content_base64/image_url 必须且只能提供一个。",
			Returns: "asset；正文使用 asset.wechat_url，封面更新文章时使用 asset.id。",
			Next:    "正文图把 wechat_url 写入 content_html；封面图把 asset.id 写入 cover_media_asset_id。",
			Check:   "不要把 media_id 填进 cover_media_asset_id；image_url 必须为公网 HTTPS，支持 JPEG/PNG/GIF/WebP 且不超过 8 MiB。",
			Fields: map[string]toolFieldRule{
				"authorizer_id":  requiredAuthorizerID(),
				"article_id":     requiredPositiveID("先创建或列出本地草稿，并使用返回的 id。"),
				"usage":          requiredStringEnum([]string{"inline_image", "cover"}, "正文图填写 inline_image，封面填写 cover。"),
				"filename":       optionalNonEmptyString("填写带扩展名的图片文件名，或省略让服务推断。", false),
				"file_path":      optionalNonEmptyString("仅限 MCP 服务器可读取且位于允许目录内的本地路径。", true),
				"content_base64": optionalNonEmptyString("填写不带 data: 前缀的 Base64 图片内容。", true),
				"image_url":      optionalHTTPSURL("填写 Agent 可访问的公网 HTTPS 图片地址。", true),
			},
			ValidateGroup: validateExactlyOneImageSource,
		},
		"official_account_publish_article": {
			Before:  "我将通过兼容接口把本地文章真实公开发布到微信；请先确认公众号、标题、摘要、正文和封面。",
			Inputs:  "必填 article_id、confirm_publish=true；文章必须有正文和有效封面 asset.id。",
			Returns: "record；保存 record.id 并同步发布状态。",
			Next:    "调用 sync_publish_status，之后用 list_published_articles 核对实时列表。",
			Check:   "若发布前校验失败，先 list_articles/update_article 补正文和封面。",
			Fields: map[string]toolFieldRule{
				"article_id":      requiredPositiveID("先调用 official_account_list_articles，并使用返回的 id。"),
				"confirm_publish": requiredTrue("用户明确确认真实发布后填写 true；不要替用户自动确认。"),
			},
		},
		"official_account_delete_article": {
			Before:  "我将删除本地文章、相关本地素材，并先删除该文章仍存在的微信已发布副本；发布记录保留审计。",
			Inputs:  "必填 article_id、confirm_delete=DELETE。",
			Returns: "deleted_article；表示微信副本清理和本地删除已完成。",
			Next:    "调用 list_articles 和 list_published_articles 确认无残留。",
			Check:   "先向用户确认这是完整删除；发布中的文章必须先同步到终态。",
			Fields: map[string]toolFieldRule{
				"article_id":     requiredPositiveID("先调用 official_account_list_articles，并使用返回的 id。"),
				"confirm_delete": requiredExactString("DELETE", "用户明确确认完整删除后填写 DELETE。"),
			},
		},
		"official_account_list_publish_records": {
			Before:  "我将读取当前用户的本地发布记录，包括 publishing、published、failed 和 deleted。",
			Inputs:  "无。",
			Returns: "items；每条包含 record id、article_id、status 和安全错误摘要。",
			Next:    "publishing 用 sync_publish_status；published 可用 delete_published_record 删除微信副本。",
			Check:   "发布记录是审计数据，不等同于微信实时文章列表。",
			Fields:  noToolFields(),
		},
		"official_account_sync_publish_status": {
			Before:  "我将向微信查询指定发布任务的最新状态，并更新本地文章和发布记录。",
			Inputs:  "必填 publish_record_id，来自 publish_draft、publish_article 或 list_publish_records。",
			Returns: "record；status 为 publishing、published、failed 或 deleted。",
			Next:    "仍为 publishing 时稍后重试；failed 时向用户展示 error_code/error_message；published 时核对实时文章列表。",
			Check:   "不要传本地 article id；必须传发布记录 id。",
			Fields: map[string]toolFieldRule{
				"publish_record_id": requiredPositiveID("先调用 official_account_list_publish_records，并使用返回的 id。"),
			},
		},
		"official_account_delete_published_record": {
			Before:  "我将只删除该发布记录对应的微信已发布副本，保留本地文章，便于修改后重新发布。",
			Inputs:  "必填 publish_record_id、confirm_delete=DELETE；记录必须为 published 且含 wechat_article_id。",
			Returns: "record；成功后 status 为 deleted。",
			Next:    "调用 list_published_articles 确认微信副本消失；需要删除本地文章时再调用 delete_article。",
			Check:   "先向用户确认删除公开内容；不要把 article_id 当作 publish_record_id。",
			Fields: map[string]toolFieldRule{
				"publish_record_id": requiredPositiveID("先调用 official_account_list_publish_records，并使用返回的 id。"),
				"confirm_delete":    requiredExactString("DELETE", "用户明确确认删除微信公开内容后填写 DELETE。"),
			},
		},
		"official_account_get_authorization_entry": {
			Before:  "我将生成绑定当前 MCP 用户的一次性公众号授权入口；链接需要由公众号管理员打开并扫码。",
			Inputs:  "无必填字段；component_appid 通常省略，使用服务端配置。",
			Returns: "authorization_entry_url 和 qr_code_payload_url；两者包含一次性 state，请勿记录或转发给无关人员。",
			Next:    "把完整链接或二维码交给用户扫码；完成后调用 list_accounts 确认公众号变为 active。",
			Check:   "链接必须以配置的首方授权域名开头，并保留完整 #authorization_url 片段。",
			Fields: map[string]toolFieldRule{
				"component_appid": optionalNonEmptyString("通常省略；仅在服务部署了多个第三方平台组件时填写。", false),
			},
		},
		"official_account_generate_authorization_url": {
			Before:  "我将生成高级一次性授权入口，可限定授权类型或预选 AppID；普通公众号授权优先使用 get_authorization_entry。",
			Inputs:  "均可选：component_appid、auth_type(1公众号/2小程序/3全部)、biz_appid；redirect_uri 通常必须省略。",
			Returns: "authorization.authorization_url；必须原样打开首方授权入口，不要提取内部微信直链。",
			Next:    "交给用户扫码后调用 list_accounts 核对授权结果。",
			Check:   "redirect_uri 只有与服务端配置回调完全一致时才允许填写；不要自定义 tenant_id 或 state。",
			Fields: map[string]toolFieldRule{
				"component_appid": optionalNonEmptyString("通常省略，使用服务端配置。", false),
				"redirect_uri":    optionalNonEmptyString("通常省略；如填写，必须与服务端授权回调完全一致。", true),
				"auth_type":       optionalIntegerEnum([]int64{0, 1, 2, 3}, "省略或填 0 使用默认；1公众号、2小程序、3全部。"),
				"biz_appid":       optionalNonEmptyString("仅在需要预选某个公众号或小程序 AppID 时填写。", false),
			},
		},
	}
}

func toolGuidanceMiddleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if method == "tools/call" {
			if call, ok := req.(*mcp.CallToolRequest); ok && call.Params != nil {
				if guide, exists := officialAccountToolGuides[call.Params.Name]; exists {
					issues := validateToolInput(call.Params.Arguments, guide)
					if len(issues) > 0 {
						return newToolInputError(call.Params.Name, guide, issues), nil
					}
				}
			}
		}

		result, err := next(ctx, method, req)
		if err != nil {
			return result, err
		}
		if method == "tools/list" {
			if listed, ok := result.(*mcp.ListToolsResult); ok {
				return decorateToolList(listed), nil
			}
		}
		if method == "tools/call" {
			call, callOK := req.(*mcp.CallToolRequest)
			toolResult, resultOK := result.(*mcp.CallToolResult)
			if callOK && call.Params != nil && resultOK && toolResult.IsError {
				if guide, exists := officialAccountToolGuides[call.Params.Name]; exists {
					return enhanceToolExecutionError(call.Params.Name, guide, toolResult), nil
				}
			}
		}
		return result, nil
	}
}

func decorateToolList(result *mcp.ListToolsResult) *mcp.ListToolsResult {
	copyResult := *result
	copyResult.Tools = make([]*mcp.Tool, 0, len(result.Tools))
	for _, tool := range result.Tools {
		if tool == nil {
			copyResult.Tools = append(copyResult.Tools, nil)
			continue
		}
		copyTool := *tool
		if guide, ok := officialAccountToolGuides[tool.Name]; ok {
			copyTool.Description = guidedDescription(tool.Description, guide)
		}
		copyResult.Tools = append(copyResult.Tools, &copyTool)
	}
	return &copyResult
}

func guidedDescription(summary string, guide toolGuide) string {
	return strings.Join([]string{
		strings.TrimSpace(summary),
		"调用前：请先用用户当前语言说明：" + guide.Before,
		"输入：" + guide.Inputs,
		"返回：" + guide.Returns,
		"后续：" + guide.Next,
		"出错时：按错误中的 field、received、expected、fix 向用户说明；缺少 ID 或确认值时先询问，不要猜测或静默重试。",
	}, "\n")
}

func validateToolInput(raw json.RawMessage, guide toolGuide) []toolInputIssue {
	arguments := make(map[string]any)
	if len(bytes.TrimSpace(raw)) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&arguments); err != nil {
			return []toolInputIssue{{
				Code: "invalid_arguments", Field: "arguments", Received: "无法解析",
				Expected: "JSON 对象", Fix: "按 tools/list 的 inputSchema 重新构造 arguments。",
			}}
		}
	}

	issues := make([]toolInputIssue, 0)
	unknown := make([]string, 0)
	for name := range arguments {
		if _, ok := guide.Fields[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	allowed := sortedFieldNames(guide.Fields)
	for _, name := range unknown {
		fix := "检查字段拼写，并改用 expected 中列出的字段。"
		if name == "tenant_id" || name == "user_id" {
			fix = "删除该字段；租户和用户范围由 MCP Token 自动确定，调用方不能覆盖。"
		}
		issues = append(issues, toolInputIssue{
			Code: "unknown_argument", Field: name, Received: "字段名不受支持",
			Expected: "允许字段：" + strings.Join(allowed, ", "),
			Fix:      fix,
		})
	}
	for _, name := range allowed {
		rule := guide.Fields[name]
		value, exists := arguments[name]
		if !exists {
			if rule.Required {
				issues = append(issues, toolInputIssue{
					Code: "missing_argument", Field: name, Received: "未提供",
					Expected: rule.Expected, Fix: rule.Fix,
				})
			}
			continue
		}
		if rule.Validate != nil && !rule.Validate(value) {
			issues = append(issues, toolInputIssue{
				Code: "invalid_argument", Field: name, Received: displayToolValue(value, rule.Sensitive),
				Expected: rule.Expected, Fix: rule.Fix,
			})
		}
	}
	if guide.ValidateGroup != nil {
		issues = append(issues, guide.ValidateGroup(arguments)...)
	}
	return issues
}

func newToolInputError(toolName string, guide toolGuide, issues []toolInputIssue) *mcp.CallToolResult {
	var message strings.Builder
	fmt.Fprintf(&message, "工具 `%s` 参数校验失败，共 %d 项：\n", toolName, len(issues))
	for i, issue := range issues {
		fmt.Fprintf(&message, "%d. `%s`：当前内容=%s；正确要求=%s；修复方式=%s\n", i+1, issue.Field, issue.Received, issue.Expected, issue.Fix)
	}
	message.WriteString("补齐或修正以上内容后再调用；不要猜测公众号、文章、素材或发布记录 ID。")
	result := new(mcp.CallToolResult)
	result.SetError(fmt.Errorf("%s", message.String()))
	result.StructuredContent = map[string]any{
		"error": map[string]any{
			"code": "invalid_tool_arguments", "tool": toolName, "issues": issues,
			"expected_inputs": guide.Inputs, "next_step": guide.Next, "retryable": true,
		},
	}
	return result
}

func enhanceToolExecutionError(toolName string, guide toolGuide, current *mcp.CallToolResult) *mcp.CallToolResult {
	raw := toolResultErrorText(current)
	code, reason := classifyToolExecutionError(raw)
	message := fmt.Sprintf("工具 `%s` 执行失败。\n原因：%s\n请检查：%s\n下一步：%s", toolName, reason, guide.Check, guide.Next)
	result := new(mcp.CallToolResult)
	result.SetError(fmt.Errorf("%s", message))
	result.StructuredContent = map[string]any{
		"error": map[string]any{
			"code": code, "tool": toolName, "reason": reason,
			"expected_inputs": guide.Inputs, "check": guide.Check, "next_step": guide.Next,
		},
	}
	return result
}

func toolResultErrorText(result *mcp.CallToolResult) string {
	if err := result.GetError(); err != nil {
		return err.Error()
	}
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok && strings.TrimSpace(text.Text) != "" {
			return text.Text
		}
	}
	return "未提供具体错误原因"
}

func classifyToolExecutionError(raw string) (string, string) {
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "internal_error"):
		return "service_error", "服务端执行异常（internal_error），敏感内部信息已隐藏。"
	case strings.Contains(lower, "invalid_request") || strings.Contains(lower, "invalid input"):
		return "invalid_request", "请求内容或资源当前状态不符合要求（invalid_request）。"
	case strings.Contains(lower, "not_found") || strings.Contains(lower, "not found"):
		return "resource_not_found", "资源不存在、已被删除，或不属于当前 MCP 用户。"
	case strings.Contains(lower, "unauthorized") || strings.Contains(lower, "returned 401"):
		return "unauthorized", "MCP Token 已失效、被撤销，或当前用户无权访问。"
	case strings.Contains(lower, "forbidden") || strings.Contains(lower, "returned 403"):
		return "forbidden", "当前用户或公众号没有执行此操作的权限。"
	case strings.Contains(lower, "conflict") || strings.Contains(lower, "returned 409"):
		return "conflict", "资源状态冲突；可能已被其他操作修改，或公众号已归属于其他用户。"
	case strings.Contains(lower, "not_implemented") || strings.Contains(lower, "not implemented"):
		return "configuration_required", "服务能力尚未配置完整，或公众号未授权对应微信权限。"
	case strings.Contains(lower, "deadline") || strings.Contains(lower, "timeout"):
		return "timeout", "调用微信或服务端超时，当前结果未确认。"
	default:
		return "tool_execution_failed", truncateToolText(raw, 500)
	}
}

func displayToolValue(value any, sensitive bool) string {
	if value == nil {
		return "null"
	}
	if sensitive {
		return "已提供（内容已隐藏）"
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("类型 %T", value)
	}
	return truncateToolText(string(raw), 120)
}

func truncateToolText(value string, limit int) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

func sortedFieldNames(fields map[string]toolFieldRule) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func noToolFields() map[string]toolFieldRule { return map[string]toolFieldRule{} }

func createContentFields() map[string]toolFieldRule {
	return map[string]toolFieldRule{
		"authorizer_id": requiredAuthorizerID(),
		"title":         requiredNonEmptyString("向用户确认文章标题后填写。", false),
		"author":        optionalString("填写作者名，或省略。", false),
		"digest":        optionalString("填写摘要，或省略让发布链路处理。", true),
		"content_html":  optionalString("填写微信兼容 HTML；创建空草稿时可省略。", true),
	}
}

func updateContentFields(idName, idFix string) map[string]toolFieldRule {
	return map[string]toolFieldRule{
		idName:                 requiredPositiveID(idFix),
		"title":                requiredNonEmptyString("向用户确认完整标题后填写。", false),
		"author":               requiredString("必须显式提供；没有作者时填写空字符串。", false),
		"digest":               requiredString("必须显式提供；没有摘要时填写空字符串。", true),
		"content_html":         requiredString("必须显式提供微信兼容 HTML；草稿阶段允许空字符串。", true),
		"cover_media_asset_id": requiredNonNegativeInteger("使用 upload_image(usage=cover) 返回的 asset.id；草稿阶段无封面可填 0。"),
	}
}

func requiredAuthorizerID() toolFieldRule {
	return requiredPositiveID("先调用 official_account_list_accounts，让用户确认公众号后使用返回的 id。")
}

func requiredPositiveID(fix string) toolFieldRule {
	return toolFieldRule{Required: true, Expected: "大于 0 的整数", Fix: fix, Validate: isPositiveInteger}
}

func requiredNonNegativeInteger(fix string) toolFieldRule {
	return toolFieldRule{Required: true, Expected: "大于等于 0 的整数", Fix: fix, Validate: isNonNegativeInteger}
}

func optionalNonNegativeInteger(fix string) toolFieldRule {
	return toolFieldRule{Expected: "大于等于 0 的整数", Fix: fix, Validate: isNonNegativeInteger}
}

func optionalDefaultedRange(minimum, maximum int64, fix string) toolFieldRule {
	return toolFieldRule{
		Expected: fmt.Sprintf("省略、0，或 %d-%d 的整数", minimum, maximum), Fix: fix,
		Validate: func(value any) bool {
			integer, ok := toolInteger(value)
			return ok && (integer == 0 || integer >= minimum && integer <= maximum)
		},
	}
}

func requiredNonEmptyString(fix string, sensitive bool) toolFieldRule {
	return toolFieldRule{Required: true, Expected: "非空字符串", Fix: fix, Sensitive: sensitive, Validate: isNonEmptyString}
}

func optionalNonEmptyString(fix string, sensitive bool) toolFieldRule {
	return toolFieldRule{Expected: "非空字符串", Fix: fix, Sensitive: sensitive, Validate: isNonEmptyString}
}

func requiredString(fix string, sensitive bool) toolFieldRule {
	return toolFieldRule{Required: true, Expected: "字符串（允许空字符串）", Fix: fix, Sensitive: sensitive, Validate: isString}
}

func optionalString(fix string, sensitive bool) toolFieldRule {
	return toolFieldRule{Expected: "字符串", Fix: fix, Sensitive: sensitive, Validate: isString}
}

func requiredExactString(expected, fix string) toolFieldRule {
	return toolFieldRule{
		Required: true, Expected: strconv.Quote(expected), Fix: fix,
		Validate: func(value any) bool { text, ok := value.(string); return ok && text == expected },
	}
}

func requiredStringEnum(values []string, fix string) toolFieldRule {
	return toolFieldRule{
		Required: true, Expected: "以下字符串之一：" + strings.Join(values, ", "), Fix: fix,
		Validate: func(value any) bool {
			text, ok := value.(string)
			if !ok {
				return false
			}
			for _, allowed := range values {
				if text == allowed {
					return true
				}
			}
			return false
		},
	}
}

func requiredTrue(fix string) toolFieldRule {
	return toolFieldRule{Required: true, Expected: "布尔值 true", Fix: fix, Validate: func(value any) bool { flag, ok := value.(bool); return ok && flag }}
}

func optionalBoolean(fix string) toolFieldRule {
	return toolFieldRule{Expected: "布尔值 true 或 false", Fix: fix, Validate: func(value any) bool { _, ok := value.(bool); return ok }}
}

func optionalIntegerEnum(values []int64, fix string) toolFieldRule {
	labels := make([]string, 0, len(values))
	allowed := make(map[int64]struct{}, len(values))
	for _, value := range values {
		labels = append(labels, strconv.FormatInt(value, 10))
		allowed[value] = struct{}{}
	}
	return toolFieldRule{
		Expected: "以下整数之一：" + strings.Join(labels, ", "), Fix: fix,
		Validate: func(value any) bool {
			integer, ok := toolInteger(value)
			_, exists := allowed[integer]
			return ok && exists
		},
	}
}

func requiredMetricsDate() toolFieldRule {
	return toolFieldRule{
		Required: true, Expected: "YYYY-MM-DD，范围 2025-11-01 到昨天",
		Fix:      "例如查询昨天；不能查询今天或未来日期。",
		Validate: func(value any) bool { return validMetricsDate(value, time.Now()) },
	}
}

func validMetricsDate(value any, now time.Time) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	china := time.FixedZone("Asia/Shanghai", 8*60*60)
	date, err := time.ParseInLocation("2006-01-02", text, china)
	if err != nil {
		return false
	}
	minimum := time.Date(2025, 11, 1, 0, 0, 0, 0, china)
	now = now.In(china)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, china)
	return !date.Before(minimum) && date.Before(today)
}

func optionalHTTPSURL(fix string, sensitive bool) toolFieldRule {
	return toolFieldRule{
		Expected: "公网 HTTPS URL", Fix: fix, Sensitive: sensitive,
		Validate: func(value any) bool {
			text, ok := value.(string)
			if !ok || strings.TrimSpace(text) == "" {
				return false
			}
			parsed, err := url.Parse(text)
			return err == nil && parsed.Scheme == "https" && parsed.Host != ""
		},
	}
}

func validateExactlyOneImageSource(arguments map[string]any) []toolInputIssue {
	sources := []string{"file_path", "content_base64", "image_url"}
	provided := make([]string, 0, len(sources))
	for _, source := range sources {
		if value, ok := arguments[source]; ok {
			if text, stringValue := value.(string); !stringValue || strings.TrimSpace(text) != "" {
				provided = append(provided, source)
			}
		}
	}
	if len(provided) == 1 {
		return nil
	}
	received := "未提供"
	if len(provided) > 0 {
		received = "同时提供了 " + strings.Join(provided, ", ")
	}
	return []toolInputIssue{{
		Code: "invalid_argument_group", Field: "file_path|content_base64|image_url", Received: received,
		Expected: "必须且只能提供一个图片来源", Fix: "线上 Agent 优先使用 image_url；Base64 使用 content_base64；服务器本地文件才使用 file_path。",
	}}
}

func isPositiveInteger(value any) bool {
	integer, ok := toolInteger(value)
	return ok && integer > 0
}

func isNonNegativeInteger(value any) bool {
	integer, ok := toolInteger(value)
	return ok && integer >= 0
}

func toolInteger(value any) (int64, bool) {
	switch number := value.(type) {
	case json.Number:
		integer, err := number.Int64()
		return integer, err == nil
	case float64:
		if math.Trunc(number) != number || number > math.MaxInt64 || number < math.MinInt64 {
			return 0, false
		}
		return int64(number), true
	case int:
		return int64(number), true
	case int64:
		return number, true
	default:
		return 0, false
	}
}

func isNonEmptyString(value any) bool {
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) != ""
}

func isString(value any) bool {
	_, ok := value.(string)
	return ok
}
