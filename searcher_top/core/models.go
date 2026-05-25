package core

import "time"

type SearchEvent struct {
	Query     string
	UserID    string
	Timestamp time.Time
}

type TopItem struct {
	Query string
	Count int64
}
