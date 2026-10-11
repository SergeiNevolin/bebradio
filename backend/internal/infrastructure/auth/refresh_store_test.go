package auth

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func testStoreWithMiniredis(t *testing.T) (*RefreshStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	return NewRefreshStore(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 30), mr
}

func testStore(t *testing.T) *RefreshStore {
	t.Helper()
	st, _ := testStoreWithMiniredis(t)
	return st
}

func TestRefreshStoreSaveAliveRevoke(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()

	if alive, err := st.Alive(ctx, "jti-1"); err != nil || alive {
		t.Fatalf("expected not alive, got %v %v", alive, err)
	}
	if err := st.Save(ctx, "jti-1", "user-1"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if alive, err := st.Alive(ctx, "jti-1"); err != nil || !alive {
		t.Fatalf("expected alive, got %v %v", alive, err)
	}
	if err := st.Revoke(ctx, "jti-1"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if alive, err := st.Alive(ctx, "jti-1"); err != nil || alive {
		t.Fatalf("expected revoked, got %v %v", alive, err)
	}
}

func TestRefreshStoreNilSafe(t *testing.T) {
	var st *RefreshStore
	if err := st.Save(context.Background(), "j", "u"); err != ErrNoStore {
		t.Errorf("nil store Save: %v", err)
	}
	if err := st.Revoke(context.Background(), "j"); err != ErrNoStore {
		t.Errorf("nil store Revoke: %v", err)
	}
	if err := st.Grace(context.Background(), "j", "u", time.Minute); err != ErrNoStore {
		t.Errorf("nil store Grace: %v", err)
	}
	if alive, err := st.Alive(context.Background(), "j"); err != ErrNoStore || alive {
		t.Errorf("nil store Alive: %v %v", alive, err)
	}
	if alive, err := NewRefreshStore(nil, 30).Alive(context.Background(), "j"); err != ErrNoStore || alive {
		t.Errorf("nil rdb Alive: %v %v", alive, err)
	}
}

func TestRefreshStoreGrace(t *testing.T) {
	st, mr := testStoreWithMiniredis(t)
	ctx := context.Background()

	if err := st.Save(ctx, "jti-g", "user-1"); err != nil {
		t.Fatalf("save: %v", err)
	}
	// мягкий отзыв: ключ жив grace-период, затем умирает сам
	if err := st.Grace(ctx, "jti-g", "user-1", time.Minute); err != nil {
		t.Fatalf("grace: %v", err)
	}
	if alive, err := st.Alive(ctx, "jti-g"); err != nil || !alive {
		t.Fatalf("expected alive during grace, got %v %v", alive, err)
	}
	mr.FastForward(61 * time.Second)
	if alive, err := st.Alive(ctx, "jti-g"); err != nil || alive {
		t.Fatalf("expected expired after grace, got %v %v", alive, err)
	}
	// мёртвый ключ grace не воскрешает
	if err := st.Grace(ctx, "jti-g", "user-1", time.Minute); err != nil {
		t.Fatalf("grace missing: %v", err)
	}
	if alive, err := st.Alive(ctx, "jti-g"); err != nil || alive {
		t.Fatalf("grace must not resurrect, got %v %v", alive, err)
	}
}
