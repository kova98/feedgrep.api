package models

type ArcticShiftSearchResponse[T any] struct {
	Data []T `json:"data"`
}

type ArcticShiftPost struct {
	ID         string `json:"id"`
	Subreddit  string `json:"subreddit"`
	Author     string `json:"author"`
	Title      string `json:"title"`
	Selftext   string `json:"selftext"`
	CreatedUTC int64  `json:"created_utc"`
}

type ArcticShiftComment struct {
	ID         string `json:"id"`
	Subreddit  string `json:"subreddit"`
	Author     string `json:"author"`
	Body       string `json:"body"`
	LinkID     string `json:"link_id"`
	ParentID   string `json:"parent_id"`
	CreatedUTC int64  `json:"created_utc"`
}
