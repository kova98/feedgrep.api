package models

type RedditListing struct {
	Data struct {
		After    string `json:"after"`
		Before   string `json:"before"`
		Children []struct {
			Data RedditPost `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

type RedditPost struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"` // fullname, e.g. "t3_abc123" or "t1_xyz789"
	Title      string  `json:"title"`
	Selftext   string  `json:"selftext"`
	Body       string  `json:"body"`
	Author     string  `json:"author"`
	Subreddit  string  `json:"subreddit"`
	Permalink  string  `json:"permalink"`
	CreatedUTC float64 `json:"created_utc"`
	LinkID     string  `json:"link_id"`
}
