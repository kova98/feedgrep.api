package models

import "time"

type Match struct {
	ID        int        `json:"id"`
	Keyword   string     `json:"keyword"`
	Source    string     `json:"source"`
	CreatedAt time.Time  `json:"createdAt"`
	Data      RedditData `json:"data"`
}

type RedditData struct {
	Subreddit string `json:"subreddit"`
	Author    string `json:"author"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Permalink string `json:"permalink"`
	IsComment bool   `json:"isComment"`
}

type GetMatchesResponse struct {
	Matches []Match `json:"matches"`
	Total   int     `json:"total"`
	Page    int     `json:"page"`
	PerPage int     `json:"perPage"`
}
