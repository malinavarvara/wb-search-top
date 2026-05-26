package db_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/malinavarvara/wb-search-top/searcher_top/adapters/db"
	"github.com/malinavarvara/wb-search-top/searcher_top/core"
)

func newRepo(windowSize, bucketSize time.Duration) *db.TopRepository {
	return db.NewTopRepository(windowSize, bucketSize)
}

func TestTopRepository_EmptyReturnsEmpty(t *testing.T) {
	r := newRepo(5*time.Minute, 10*time.Second)
	top := r.GetTop(10)
	if len(top) != 0 {
		t.Fatalf("empty repo: want 0 items, got %d", len(top))
	}
}

func TestTopRepository_SingleEvent(t *testing.T) {
	r := newRepo(5*time.Minute, 10*time.Second)
	r.Increment("кроссовки", "u1", time.Now())

	top := r.GetTop(10)
	if len(top) != 1 {
		t.Fatalf("want 1 item, got %d", len(top))
	}
	assertItem(t, top[0], "кроссовки", 1)
}

func TestTopRepository_TopSortedByCount(t *testing.T) {
	r := newRepo(5*time.Minute, 10*time.Second)

	// платье — 3 уникальных юзера
	r.Increment("платье", "u1", time.Now())
	r.Increment("платье", "u2", time.Now())
	r.Increment("платье", "u3", time.Now())
	// кроссовки — 2 уникальных юзера
	r.Increment("кроссовки", "u1", time.Now())
	r.Increment("кроссовки", "u2", time.Now())
	// найк — 1 юзер
	r.Increment("найк", "u1", time.Now())

	top := r.GetTop(3)
	if len(top) != 3 {
		t.Fatalf("want 3 items, got %d", len(top))
	}
	assertItem(t, top[0], "платье", 3)
	assertItem(t, top[1], "кроссовки", 2)
	assertItem(t, top[2], "найк", 1)
}

func TestTopRepository_LimitN(t *testing.T) {
	r := newRepo(5*time.Minute, 10*time.Second)
	for i := 0; i < 10; i++ {
		r.Increment(fmt.Sprintf("query-%d", i), fmt.Sprintf("u%d", i), time.Now())
	}

	top := r.GetTop(3)
	if len(top) != 3 {
		t.Fatalf("want 3 items (limited), got %d", len(top))
	}
}

func TestTopRepository_NZeroReturnsAll(t *testing.T) {
	r := newRepo(5*time.Minute, 10*time.Second)
	r.Increment("a", "u1", time.Now())
	r.Increment("b", "u2", time.Now())
	r.Increment("c", "u3", time.Now())

	top := r.GetTop(0)
	if len(top) != 3 {
		t.Fatalf("n=0: want all 3 items, got %d", len(top))
	}
}

func TestTopRepository_SameUserSameQueryCountsOnce(t *testing.T) {
	r := newRepo(5*time.Minute, 10*time.Second)

	// 100 раз в одном бакете — должен считаться как 1
	for i := 0; i < 100; i++ {
		r.Increment("спам", "bot-1", time.Now())
	}

	top := r.GetTop(1)
	if len(top) == 0 {
		t.Fatal("expected at least 1 item")
	}
	assertItem(t, top[0], "спам", 1)
}

func TestTopRepository_DifferentUsersCountSeparately(t *testing.T) {
	r := newRepo(5*time.Minute, 10*time.Second)
	r.Increment("найк", "u1", time.Now())
	r.Increment("найк", "u2", time.Now())
	r.Increment("найк", "u3", time.Now())

	top := r.GetTop(1)
	assertItem(t, top[0], "найк", 3)
}

func TestTopRepository_SameUserDifferentQueriesCountedSeparately(t *testing.T) {
	r := newRepo(5*time.Minute, 10*time.Second)
	r.Increment("кроссовки", "u1", time.Now())
	r.Increment("платье", "u1", time.Now())

	top := r.GetTop(10)
	if len(top) != 2 {
		t.Fatalf("want 2 items, got %d", len(top))
	}
}

func TestTopRepository_EqualCountsSortedLexicographically(t *testing.T) {
	r := newRepo(5*time.Minute, 10*time.Second)
	r.Increment("яблоко", "u1", time.Now())
	r.Increment("абрикос", "u2", time.Now())

	top := r.GetTop(2)
	if len(top) != 2 {
		t.Fatalf("want 2 items, got %d", len(top))
	}
	if top[0].Query != "абрикос" {
		t.Errorf("equal counts: want 'абрикос' first (lexicographic), got %q", top[0].Query)
	}
}

func TestTopRepository_RotationExpiresBucket(t *testing.T) {
	bucketSize := 100 * time.Millisecond
	windowSize := 200 * time.Millisecond
	r := newRepo(windowSize, bucketSize)

	r.Increment("старый запрос", "u1", time.Now())

	top := r.GetTop(10)
	if len(top) == 0 {
		t.Fatal("want 1 item before rotation")
	}

	time.Sleep(windowSize + bucketSize + 50*time.Millisecond)

	top = r.GetTop(10)
	if len(top) != 0 {
		t.Errorf("after full window rotation: want 0 items, got %d (%v)", len(top), top)
	}
}

func TestTopRepository_NewEventsAfterRotationVisible(t *testing.T) {
	bucketSize := 100 * time.Millisecond
	windowSize := 300 * time.Millisecond
	r := newRepo(windowSize, bucketSize)

	r.Increment("старый", "u1", time.Now())

	time.Sleep(windowSize + bucketSize + 50*time.Millisecond)

	r.Increment("новый", "u2", time.Now())

	top := r.GetTop(10)
	if len(top) != 1 {
		t.Fatalf("want 1 item (only new), got %d: %v", len(top), top)
	}
	if top[0].Query != "новый" {
		t.Errorf("want 'новый', got %q", top[0].Query)
	}
}

func TestTopRepository_UserAcrossMultipleBucketsCountedOnce(t *testing.T) {
	bucketSize := 100 * time.Millisecond
	windowSize := 500 * time.Millisecond
	r := newRepo(windowSize, bucketSize)

	r.Increment("найк", "u1", time.Now())
	time.Sleep(bucketSize + 20*time.Millisecond)
	r.Increment("найк", "u1", time.Now())

	top := r.GetTop(1)
	if len(top) == 0 {
		t.Fatal("expected item in top")
	}
	assertItem(t, top[0], "найк", 1)
}

func TestTopRepository_ConcurrentWritesSafe(t *testing.T) {
	r := newRepo(5*time.Minute, 10*time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r.Increment("concurrent", fmt.Sprintf("u%d", n), time.Now())
		}(i)
	}
	wg.Wait()

	top := r.GetTop(1)
	if len(top) == 0 {
		t.Fatal("expected items after concurrent writes")
	}
	if top[0].Count != 200 {
		t.Errorf("want count=200, got %d", top[0].Count)
	}
}

func TestTopRepository_ConcurrentReadWriteSafe(t *testing.T) {
	r := newRepo(5*time.Minute, 10*time.Second)

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r.Increment("query", fmt.Sprintf("u%d", n), time.Now())
		}(i)
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.GetTop(10)
		}()
	}

	wg.Wait()
}

func assertItem(t *testing.T, item core.TopItem, wantQuery string, wantCount int64) {
	t.Helper()
	if item.Query != wantQuery {
		t.Errorf("query: want %q, got %q", wantQuery, item.Query)
	}
	if item.Count != wantCount {
		t.Errorf("count for %q: want %d, got %d", wantQuery, wantCount, item.Count)
	}
}
