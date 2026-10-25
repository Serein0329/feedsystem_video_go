package feed

type FeedAuthor struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

type FeedVideoItem struct {
	ID          uint       `json:"id"`
	Author      FeedAuthor `json:"author"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	PlayURL     string     `json:"play_url"`
	CoverURL    string     `json:"cover_url"`
	CreateTime  int64      `json:"create_time"`
	LikesCount  int64      `json:"likes_count"`
	IsLiked     bool       `json:"is_liked"`
}

type ListLatestRequest struct {
	Limit      int   `json:"limit"`
	LatestTime int64 `json:"latest_time"`
}

type ListLatestResponse struct {
	VideoList []FeedVideoItem `json:"video_list"`
	NextTime  int64           `json:"next_time"`
	HasMore   bool            `json:"has_more"`
}

type ListLikesCountRequest struct {
	Limit          int    `json:"limit"`
	LastLikesCount *int64 `json:"last_likes_count" binding:"omitempty,min=0"`
	LastID         *uint  `json:"last_id" binding:"omitempty,min=1"`
}

type ListLikesCountResponse struct {
	VideoList      []FeedVideoItem `json:"video_list"`
	NextLikesCount *int64          `json:"next_likes_count,omitempty"`
	NextID         *uint           `json:"next_id,omitempty"`
	HasMore        bool            `json:"has_more"`
}

type ListByFollowingRequest struct {
	Limit    int  `json:"limit"`
	LatestID uint `json:"latest_id"`
}

type ListByFollowingResponse struct {
	VideoList []FeedVideoItem `json:"video_list"`
	NextID    uint            `json:"next_id"`
	HasMore   bool            `json:"has_more"`
}

type ListByPopularityRequest struct {
	Limit  int   `json:"limit"`
	Offset int64 `json:"offset"`
	Asof   int64 `json:"asof"`
}

type ListByPopularityResponse struct {
	VideoList  []FeedVideoItem `json:"video_list"`
	NextOffset int64           `json:"next_offset"`
	Asof       int64           `json:"asof"`
	HasMore    bool            `json:"has_more"`
}

type ListByTagRequest struct {
	TagName string `json:"tag_name" binding:"required"`
	Limit   int    `json:"limit"`
	LastID  uint   `json:"last_id"`
}

type ListByTagResponse struct {
	VideoList []FeedVideoItem `json:"video_list"`
	NextID    uint            `json:"next_id"`
	HasMore   bool            `json:"has_more"`
}
