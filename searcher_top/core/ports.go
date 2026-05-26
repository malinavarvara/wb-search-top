package core

import "context"

type SearchService interface {
	ProcessEvent(ctx context.Context, event SearchEvent) error
	GetTop(ctx context.Context, n int) ([]TopItem, error)
	AddStopWord(ctx context.Context, word string) error
	RemoveStopWord(ctx context.Context, word string) error
	GetStopWords(ctx context.Context) ([]string, error)
}
