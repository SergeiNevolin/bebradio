package auth

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func testStore(t *testing.T) *RefreshStore {
	t.Helper()
	mr := miniredis.RunT(t)
	return NewRefreshStore(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 30)
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
	if alive, err := st.Alive(context.Background(), "j"); err != ErrNoStore || alive {
		t.Errorf("nil store Alive: %v %v", alive, err)
	}
	if alive, err := NewRefreshStore(nil, 30).Alive(context.Background(), "j"); err != ErrNoStore || alive {
		t.Errorf("nil rdb Alive: %v %v", alive, err)
	}
}
