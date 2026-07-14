package wechat

import (
	"context"

	"official-account-service/internal/domain/officialcontent"
)

// ListPublishedArticles reads one live page from WeChat freepublish/batchget.
func (p *Publisher) ListPublishedArticles(ctx context.Context, accessToken string, offset, count int, includeContent bool) (officialcontent.PublishedArticleBatch, error) {
	var response freePublishBatchGetResponse
	noContent := 1
	if includeContent {
		noContent = 0
	}
	if err := p.postJSON(ctx, "freepublish_batchget", "/freepublish/batchget", accessToken, freePublishBatchGetRequest{
		Offset: offset, Count: count, NoContent: noContent,
	}, &response); err != nil {
		return officialcontent.PublishedArticleBatch{}, err
	}
	items := make([]officialcontent.PublishedArticle, 0)
	for _, message := range response.Items {
		for index, article := range message.Content.NewsItems {
			items = append(items, officialcontent.PublishedArticle{
				ArticleID: message.ArticleID, Index: index, UpdateTime: message.UpdateTime,
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
