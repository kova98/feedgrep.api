package models

type DigestItem struct {
	Keyword   string
	Title     string
	Subreddit string
	URL       string
}

type Digest struct {
	Items []DigestItem
	Total int
	IDs   []int64
}

type Email struct {
	To      string
	Subject string
	Body    string
}
