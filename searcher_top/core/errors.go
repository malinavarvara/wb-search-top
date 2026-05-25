package core

import "errors"

var (
	ErrEmptyQuery   = errors.New("query must not be empty")
	ErrEmptyWord    = errors.New("stop word must not be empty")
	ErrInvalidLimit = errors.New("limit must be between 1 and max_top_limit")
)
