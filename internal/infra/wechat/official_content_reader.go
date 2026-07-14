package wechat

import (
	"context"
	"net/url"
	"strings"

	"official-account-service/internal/domain/officialcontent"
)

const maxOfficialContentResponseBytes int64 = 8 << 20

var _ officialcontent.ContentReader = (*Publisher)(nil)

// ListPublishedArticles reads one live page from WeChat freepublish/batchget.
func (p *Publisher) ListPublishedArticles(ctx context.Context, accessToken string, offset, count int, includeContent bool) (officialcontent.PublishedArticleBatch, error) {
	var response freePublishBatchGetResponse
	noContent := 1
	if includeContent {
		noContent = 0
	}
	if err := p.postJSONWithResponseLimit(ctx, "freepublish_batchget", "/freepublish/batchget", accessToken, freePublishBatchGetRequest{
		Offset: offset, Count: count, NoContent: noContent,
	}, &response, maxOfficialContentResponseBytes); err != nil {
		return officialcontent.PublishedArticleBatch{}, err
	}
	items := make([]officialcontent.PublishedArticle, 0)
	for _, message := range response.Items {
		for index, article := range message.Content.NewsItems {
			items = append(items, officialcontent.PublishedArticle{
				ArticleID: message.ArticleID, MsgID: publishedArticleMsgID(article.URL),
				Index: index, UpdateTime: message.UpdateTime,
				Title: article.Title, Author: article.Author, Digest: article.Digest,
				ContentHTML: article.Content, ContentSourceURL: article.ContentSourceURL,
				ThumbMediaID: article.ThumbMediaID, ThumbURL: article.ThumbURL, URL: article.URL,
				NeedOpenComment:    article.NeedOpenComment == 1,
				OnlyFansCanComment: article.OnlyFansCanComment == 1,
				Deleted:            article.IsDeleted,
			})
		}
	}
	return officialcontent.PublishedArticleBatch{
		TotalMessageCount: response.TotalCount, FetchedMessageCount: response.ItemCount, Items: items,
	}, nil
}

func publishedArticleMsgID(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	mid := strings.TrimSpace(parsed.Query().Get("mid"))
	index := strings.TrimSpace(parsed.Query().Get("idx"))
	if !isDecimalDigits(mid) || !isDecimalDigits(index) || index == "0" {
		return ""
	}
	return mid + "_" + index
}

// GetArticleMetrics reads live cumulative article metrics for one publish date.
func (p *Publisher) GetArticleMetrics(ctx context.Context, accessToken, date string) (officialcontent.ArticleMetricsResult, error) {
	var response articleMetricsResponse
	if err := p.postJSONFromAPIRoot(ctx, "getarticletotaldetail", "/datacube/getarticletotaldetail", accessToken, articleMetricsRequest{
		BeginDate: date, EndDate: date,
	}, &response); err != nil {
		return officialcontent.ArticleMetricsResult{}, err
	}
	return officialcontent.ArticleMetricsResult{Date: date, Delayed: response.IsDelay, Items: response.Items}, nil
}

// ListArticleComments reads one live comment page without exposing OpenIDs.
func (p *Publisher) ListArticleComments(ctx context.Context, accessToken string, msgDataID int64, articleIndex, begin, count, commentType int) (officialcontent.ArticleCommentBatch, error) {
	var response articleCommentsResponse
	if err := p.postJSON(ctx, "comment_list", "/comment/list", accessToken, articleCommentsRequest{
		MsgDataID: msgDataID, Index: articleIndex, Begin: begin, Count: count, Type: commentType,
	}, &response); err != nil {
		return officialcontent.ArticleCommentBatch{}, err
	}
	items := make([]officialcontent.ArticleComment, 0, len(response.Comments))
	for _, comment := range response.Comments {
		item := officialcontent.ArticleComment{
			UserCommentID: comment.UserCommentID, CreateTime: comment.CreateTime,
			Content: comment.Content, Selected: comment.CommentType == 1,
		}
		if comment.Reply != nil {
			item.Reply = &officialcontent.CommentReply{Content: comment.Reply.Content, CreateTime: comment.Reply.CreateTime}
		}
		items = append(items, item)
	}
	return officialcontent.ArticleCommentBatch{Total: response.Total, Items: items}, nil
}

func (p *Publisher) postJSONFromAPIRoot(ctx context.Context, operation, path, accessToken string, body any, out wechatPublishResponse) error {
	client := *p
	client.baseURL = strings.TrimSuffix(client.baseURL, "/cgi-bin")
	return client.postJSONWithResponseLimit(ctx, operation, path, accessToken, body, out, maxOfficialContentResponseBytes)
}

type freePublishBatchGetRequest struct {
	Offset    int `json:"offset"`
	Count     int `json:"count"`
	NoContent int `json:"no_content"`
}

type freePublishBatchGetResponse struct {
	TotalCount int                           `json:"total_count"`
	ItemCount  int                           `json:"item_count"`
	Items      []freePublishPublishedMessage `json:"item"`
	ErrCode    int                           `json:"errcode"`
	ErrMsg     string                        `json:"errmsg"`
}

func (r freePublishBatchGetResponse) wechatError(operation string) error {
	return wechatPublishError(operation, r.ErrCode, r.ErrMsg)
}

type freePublishPublishedMessage struct {
	ArticleID  string `json:"article_id"`
	UpdateTime int64  `json:"update_time"`
	Content    struct {
		NewsItems []freePublishPublishedArticle `json:"news_item"`
	} `json:"content"`
}

type freePublishPublishedArticle struct {
	Title              string `json:"title"`
	Author             string `json:"author"`
	Digest             string `json:"digest"`
	Content            string `json:"content"`
	ContentSourceURL   string `json:"content_source_url"`
	ThumbMediaID       string `json:"thumb_media_id"`
	ThumbURL           string `json:"thumb_url"`
	NeedOpenComment    int    `json:"need_open_comment"`
	OnlyFansCanComment int    `json:"only_fans_can_comment"`
	URL                string `json:"url"`
	IsDeleted          bool   `json:"is_deleted"`
}

type articleMetricsRequest struct {
	BeginDate string `json:"begin_date"`
	EndDate   string `json:"end_date"`
}

type articleMetricsResponse struct {
	Items   []officialcontent.ArticleMetrics `json:"list"`
	IsDelay bool                             `json:"is_delay"`
	ErrCode int                              `json:"errcode"`
	ErrMsg  string                           `json:"errmsg"`
}

func (r articleMetricsResponse) wechatError(operation string) error {
	return wechatPublishError(operation, r.ErrCode, r.ErrMsg)
}

type articleCommentsRequest struct {
	MsgDataID int64 `json:"msg_data_id"`
	Index     int   `json:"index"`
	Begin     int   `json:"begin"`
	Count     int   `json:"count"`
	Type      int   `json:"type"`
}

type articleCommentsResponse struct {
	Total    int                 `json:"total"`
	Comments []wechatCommentItem `json:"comment"`
	ErrCode  int                 `json:"errcode"`
	ErrMsg   string              `json:"errmsg"`
}

func (r articleCommentsResponse) wechatError(operation string) error {
	return wechatPublishError(operation, r.ErrCode, r.ErrMsg)
}

type wechatCommentItem struct {
	UserCommentID int64               `json:"user_comment_id"`
	CreateTime    int64               `json:"create_time"`
	Content       string              `json:"content"`
	CommentType   int                 `json:"comment_type"`
	Reply         *wechatCommentReply `json:"reply"`
}

type wechatCommentReply struct {
	Content    string `json:"content"`
	CreateTime int64  `json:"create_time"`
}
