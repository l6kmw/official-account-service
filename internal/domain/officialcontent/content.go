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

// ArticleMetrics contains one published article's cumulative detail series.
type ArticleMetrics struct {
	RefDate     string                `json:"ref_date"`
	MsgID       string                `json:"msgid"`
	PublishType int                   `json:"publish_type"`
	Title       string                `json:"title"`
	ContentURL  string                `json:"content_url"`
	Details     []ArticleMetricDetail `json:"detail_list"`
}

// ArticleMetricDetail contains cumulative metrics through one statistics date.
type ArticleMetricDetail struct {
	StatDate          string             `json:"stat_date"`
	ReadUser          int64              `json:"read_user"`
	ReadUserSources   []ReadUserSource   `json:"read_user_source"`
	ShareUser         int64              `json:"share_user"`
	ZaikanUser        int64              `json:"zaikan_user"`
	LikeUser          int64              `json:"like_user"`
	CommentCount      int64              `json:"comment_count"`
	CollectionUser    int64              `json:"collection_user"`
	PraiseMoney       int64              `json:"praise_money"`
	ReadSubscribeUser int64              `json:"read_subscribe_user"`
	ReadDeliveryRate  float64            `json:"read_delivery_rate"`
	ReadFinishRate    float64            `json:"read_finish_rate"`
	ReadAvgActiveTime float64            `json:"read_avg_activetime"`
	ReadJumpPositions []ReadJumpPosition `json:"read_jump_position"`
}

type ReadUserSource struct {
	UserCount int64  `json:"user_count"`
	SceneDesc string `json:"scene_desc"`
}

type ReadJumpPosition struct {
	Position int     `json:"position"`
	Rate     float64 `json:"rate"`
}

// ArticleMetricsResult is the live WeChat metrics response for one publish date.
type ArticleMetricsResult struct {
	Date    string           `json:"date"`
	Delayed bool             `json:"delayed"`
	Items   []ArticleMetrics `json:"items"`
}

// MetricsReader reads live article metrics from WeChat.
type MetricsReader interface {
	GetArticleMetrics(ctx context.Context, accessToken, date string) (ArticleMetricsResult, error)
}

// ArticleComment is a WeChat article comment without user OpenID.
type ArticleComment struct {
	UserCommentID int64         `json:"user_comment_id"`
	CreateTime    int64         `json:"create_time"`
	Content       string        `json:"content"`
	Selected      bool          `json:"selected"`
	Reply         *CommentReply `json:"reply,omitempty"`
}

type CommentReply struct {
	Content    string `json:"content"`
	CreateTime int64  `json:"create_time"`
}

// ArticleCommentBatch is one raw WeChat comment page.
type ArticleCommentBatch struct {
	Total int              `json:"total"`
	Items []ArticleComment `json:"items"`
}

// ArticleCommentList adds safe pagination and article identity metadata.
type ArticleCommentList struct {
	MsgID         string           `json:"msgid"`
	MsgDataID     int64            `json:"msg_data_id"`
	ArticleIndex  int              `json:"article_index"`
	Total         int              `json:"total"`
	ReturnedCount int              `json:"returned_count"`
	NextBegin     int              `json:"next_begin"`
	HasMore       bool             `json:"has_more"`
	Items         []ArticleComment `json:"items"`
}

// CommentReader reads live article comments from WeChat.
type CommentReader interface {
	ListArticleComments(ctx context.Context, accessToken string, msgDataID int64, articleIndex, begin, count, commentType int) (ArticleCommentBatch, error)
}
