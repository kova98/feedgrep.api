package models

type RedditListing struct {
	Data struct {
		Children []struct {
			Data RedditPost `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

type RedditPost struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Selftext   string  `json:"selftext"`
	Body       string  `json:"body"`
	Author     string  `json:"author"`
	Subreddit  string  `json:"subreddit"`
	Permalink  string  `json:"permalink"`
	CreatedUTC float64 `json:"created_utc"`
	LinkID     string  `json:"link_id"`
}
