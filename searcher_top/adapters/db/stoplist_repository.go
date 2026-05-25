package db

import (
	"strings"
	"sync"
)

type StopListRepository struct {
	mu    sync.RWMutex
	words map[string]struct{}
}

func NewStopListRepository() *StopListRepository {
	return &StopListRepository{
		words: make(map[string]struct{}),
	}
}

func (s *StopListRepository) Add(word string) {
	word = normalize(word)
	if word == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.words[word] = struct{}{}
}

func (s *StopListRepository) Remove(word string) {
	word = normalize(word)
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.words, word)
}

func (s *StopListRepository) Contains(word string) bool {
	word = normalize(word)
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.words[word]
	return ok
}

func (s *StopListRepository) GetAll() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, 0, len(s.words))
	for w := range s.words {
		result = append(result, w)
	}
	return result
}

func normalize(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}
