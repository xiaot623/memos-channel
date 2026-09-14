package core

import (
	"fmt"
	"testing"
	"time"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
)

func TestGroupCacheExpires(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cache := newGroupCache(time.Minute)
	cache.now = func() time.Time { return now }

	created := 0
	create := func() (*v1pb.Memo, error) {
		created++
		return &v1pb.Memo{Name: fmt.Sprintf("memos/%d", created)}, nil
	}

	first, err := cache.getOrCreate("album", create)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(30 * time.Second)
	second, err := cache.getOrCreate("album", create)
	if err != nil {
		t.Fatal(err)
	}
	if first.Name != second.Name || created != 1 {
		t.Fatalf("expected cache reuse, created=%d first=%s second=%s", created, first.Name, second.Name)
	}

	now = now.Add(31 * time.Second)
	third, err := cache.getOrCreate("album", create)
	if err != nil {
		t.Fatal(err)
	}
	if third.Name == first.Name || created != 2 {
		t.Fatalf("expected TTL miss, created=%d third=%s", created, third.Name)
	}
}
