package db_test

import (
	"sync"
	"testing"

	"github.com/malinavarvara/wb-search-top/searcher_top/adapters/db"
)

func TestStopList_AddAndContains(t *testing.T) {
	s := db.NewStopListRepository()
	s.Add("наркотики")
	if !s.Contains("наркотики") {
		t.Error("Contains must return true after Add")
	}
}

func TestStopList_ContainsFalseForMissing(t *testing.T) {
	s := db.NewStopListRepository()
	if s.Contains("несуществующее") {
		t.Error("Contains must return false for unknown word")
	}
}

func TestStopList_Remove(t *testing.T) {
	s := db.NewStopListRepository()
	s.Add("слово")
	s.Remove("слово")
	if s.Contains("слово") {
		t.Error("Contains must return false after Remove")
	}
}

func TestStopList_RemoveNonExistentIsIdempotent(t *testing.T) {
	s := db.NewStopListRepository()
	s.Remove("нет такого")
}

func TestStopList_AddIdempotent(t *testing.T) {
	s := db.NewStopListRepository()
	s.Add("дубль")
	s.Add("дубль")
	s.Add("дубль")

	all := s.GetAll()
	if len(all) != 1 {
		t.Errorf("idempotent add: want 1, got %d", len(all))
	}
}

func TestStopList_NormalizesOnAdd(t *testing.T) {
	s := db.NewStopListRepository()
	s.Add("  СПАМ  ")

	if !s.Contains("спам") {
		t.Error("want lowercase+trimmed match: 'спам'")
	}
}

func TestStopList_NormalizesOnContains(t *testing.T) {
	s := db.NewStopListRepository()
	s.Add("слово")

	cases := []string{"СЛОВО", "Слово", "  слово  ", "  СЛОВО  "}
	for _, c := range cases {
		if !s.Contains(c) {
			t.Errorf("Contains(%q) = false, want true", c)
		}
	}
}

func TestStopList_NormalizesOnRemove(t *testing.T) {
	s := db.NewStopListRepository()
	s.Add("слово")
	s.Remove("  СЛОВО  ") // удаляем в другом регистре
	if s.Contains("слово") {
		t.Error("word must be removed after normalized Remove")
	}
}

func TestStopList_EmptyWordIgnored(t *testing.T) {
	s := db.NewStopListRepository()
	s.Add("")
	s.Add("   ")

	all := s.GetAll()
	if len(all) != 0 {
		t.Errorf("empty words must not be stored, got %d items: %v", len(all), all)
	}
}

func TestStopList_GetAllReturnsSnapshot(t *testing.T) {
	s := db.NewStopListRepository()
	s.Add("a")
	s.Add("b")
	s.Add("c")

	all := s.GetAll()
	if len(all) != 3 {
		t.Fatalf("want 3 words, got %d", len(all))
	}

	s.Add("d")
	if len(all) != 3 {
		t.Error("GetAll must return snapshot, not a live slice")
	}
}

func TestStopList_GetAllEmptyIsNotNil(t *testing.T) {
	s := db.NewStopListRepository()
	all := s.GetAll()
	if all == nil {
		t.Error("GetAll on empty store must return empty slice, not nil")
	}
}

func TestStopList_ConcurrentAddContainsSafe(t *testing.T) {
	s := db.NewStopListRepository()

	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			s.Add("слово")
		}(i)
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.Contains("слово")
		}()
	}

	wg.Wait()
}

func TestStopList_ConcurrentAddRemoveSafe(t *testing.T) {
	s := db.NewStopListRepository()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.Add("contested")
		}()
		go func() {
			defer wg.Done()
			s.Remove("contested")
		}()
	}
	wg.Wait()
}
