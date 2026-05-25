package db

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/malinavarvara/wb-search-top/searcher_top/core"
)

type bucket struct {
	users map[string]map[string]struct{} // query → set users
	start time.Time                      //возможно удалить, если не понадобится для отладки
}

func newBucket(t time.Time) *bucket {
	return &bucket{
		users: make(map[string]map[string]struct{}),
		start: t,
	}
}

func (b *bucket) add(query, userID string) {
	if _, ok := b.users[query]; !ok {
		b.users[query] = make(map[string]struct{})
	}
	b.users[query][userID] = struct{}{}
}

type TopRepository struct {
	mu         sync.RWMutex
	buckets    []*bucket
	bucketSize time.Duration // длина одного бакета (10s)
	windowSize time.Duration // общий размор окна (5m)
	numBuckets int           // 30 = 5m / 10s
}

func NewTopRepository(windowSize, bucketSize time.Duration) *TopRepository {
	n := int(windowSize / bucketSize)
	buckets := make([]*bucket, n)
	now := time.Now()
	for i := range buckets {
		buckets[i] = newBucket(now)
	}
	r := &TopRepository{
		buckets:    buckets,
		bucketSize: bucketSize,
		windowSize: windowSize,
		numBuckets: n,
	}
	go r.rotateLoop()
	return r
}

func (r *TopRepository) Increment(query, userID string, _ time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	current := r.buckets[r.numBuckets-1]
	current.add(query, userID)
}

func (r *TopRepository) GetTop(n int) []core.TopItem {
	r.mu.RLock()
	defer r.mu.RUnlock()

	merged := make(map[string]map[string]struct{})
	for _, b := range r.buckets {
		for query, users := range b.users {
			if _, ok := merged[query]; !ok {
				merged[query] = make(map[string]struct{})
			}
			for uid := range users {
				merged[query][uid] = struct{}{}
			}
		}
	}

	items := make([]core.TopItem, 0, len(merged))
	for query, users := range merged {
		count := int64(len(users))
		items = append(items, core.TopItem{
			Query: query,
			Count: count,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return strings.Compare(items[i].Query, items[j].Query) < 0
	})

	if n > 0 && n < len(items) {
		return items[:n]
	}
	return items
}

func (r *TopRepository) rotateLoop() {
	ticker := time.NewTicker(r.bucketSize)
	defer ticker.Stop()
	for t := range ticker.C {
		r.rotate(t)
	}
}

func (r *TopRepository) rotate(t time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	copy(r.buckets, r.buckets[1:])
	r.buckets[r.numBuckets-1] = newBucket(t)
}
