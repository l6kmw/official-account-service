package officialcontent

import "context"

// PublishedArticle is one article inside a WeChat published message.
type PublishedArticle struct {
	ArticleID          string `json:"article_id"`
	Index              int    `json:"index"`
	UpdateTime         int64  `json:"update_time"`
	Title              string `json:"title"`
	Author             string `json:"author"`
	Digest             string `json:"digest"`
	ContentHTML        string `json:"content_html,omitempty"`
	ContentSourceURL   string `json:"content_source_url"`
	ThumbMediaID       string `json:"thumb_media_id"`
	ThumbURL           string `json:"thumb_url"`
	URL                string `json:"url"`
	NeedOpenComment    bool   `json:"need_open_comment"`
	OnlyFansCanComment bool   `json:"only_fans_can_comment"`
	Deleted            bool   `json:"deleted"`
}

// PublishedArticleBatch is one raw WeChat message page expanded into articles.
type PublishedArticleBatch struct {
	TotalMessageCount   int                `json:"total_message_count"`
	FetchedMessageCount int                `json:"fetched_message_count"`
	Items               []PublishedArticle `json:"items"`
}

// PublishedArticleList adds stable message pagination metadata to a batch.
type PublishedArticleList struct {
	TotalMessageCount    int                `json:"total_message_count"`
	FetchedMessageCount  int                `json:"fetched_message_count"`
	ReturnedArticleCount int                `json:"returned_article_count"`
	NextOffset           int                `json:"next_offset"`
	HasMore              bool               `json:"has_more"`
	Items                []PublishedArticle `json:"items"`
}

// Reader reads live official-account content from WeChat.
type Reader interface {
	ListPublishedArticles(ctx context.Context, accessToken string, offset, count int, includeContent bool) (PublishedArticleBatch, error)
}
