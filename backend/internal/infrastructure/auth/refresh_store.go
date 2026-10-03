package auth

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrNoStore — Redis недоступен (или не подключён): refresh-сессии не работают.
var ErrNoStore = errors.New("refresh store unavailable")

const refreshKeyPrefix = "auth:refresh:"

// RefreshStore — живые refresh-токены в Redis (TTL = сроку токена).
// Ротация = Save(новый) + Revoke(старый); повтор старого jti = 401.
type RefreshStore struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewRefreshStore(rdb *redis.Client, refreshDays int) *RefreshStore {
	if refreshDays <= 0 {
		refreshDays = 30
	}
	return &RefreshStore{rdb: rdb, ttl: time.Duration(refreshDays) * 24 * time.Hour}
}

func (st *RefreshStore) Save(ctx context.Context, jti, userID string) error {
	if st == nil || st.rdb == nil {
		return ErrNoStore
	}
	return st.rdb.Set(ctx, refreshKeyPrefix+jti, userID, st.ttl).Err()
}

func (st *RefreshStore) Revoke(ctx context.Context, jti string) error {
	if st == nil || st.rdb == nil {
		return ErrNoStore
	}
	return st.rdb.Del(ctx, refreshKeyPrefix+jti).Err()
}

// Alive — токен не отозван и не истёк по TTL.
func (st *RefreshStore) Alive(ctx context.Context, jti string) (bool, error) {
	if st == nil || st.rdb == nil {
		return false, ErrNoStore
	}
	n, err := st.rdb.Exists(ctx, refreshKeyPrefix+jti).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
