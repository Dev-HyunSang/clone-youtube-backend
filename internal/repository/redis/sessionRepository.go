package redis

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

var ErrSessionNotFound = errors.New("세션을 찾을 수 없습니다")

// SessionRepository tracks one Redis key per logged-in device ("session"),
// keyed by user + session ID, valued by the session's current refresh-token
// jti. A per-user set indexes session IDs so all of a user's devices can be
// revoked at once.
type SessionRepository interface {
	// Create stores/rotates the session's current refresh-token id.
	Create(userID, sessionID, refreshTokenID string, ttl time.Duration) error
	// Get returns the session's current refresh-token id, or ErrSessionNotFound if revoked/expired.
	Get(userID, sessionID string) (string, error)
	Delete(userID, sessionID string) error
	DeleteAll(userID string) error
}

type sessionRepository struct {
	rdb *goredis.Client
}

func NewSessionRepository(rdb *goredis.Client) SessionRepository {
	return &sessionRepository{rdb: rdb}
}

func sessionKey(userID, sessionID string) string {
	return "session:" + userID + ":" + sessionID
}

func sessionSetKey(userID string) string {
	return "sessions:" + userID
}

func (r *sessionRepository) Create(userID, sessionID, refreshTokenID string, ttl time.Duration) error {
	ctx := context.Background()
	if err := r.rdb.Set(ctx, sessionKey(userID, sessionID), refreshTokenID, ttl).Err(); err != nil {
		return err
	}
	return r.rdb.SAdd(ctx, sessionSetKey(userID), sessionID).Err()
}

func (r *sessionRepository) Get(userID, sessionID string) (string, error) {
	val, err := r.rdb.Get(context.Background(), sessionKey(userID, sessionID)).Result()
	if errors.Is(err, goredis.Nil) {
		return "", ErrSessionNotFound
	}
	return val, err
}

func (r *sessionRepository) Delete(userID, sessionID string) error {
	ctx := context.Background()
	if err := r.rdb.Del(ctx, sessionKey(userID, sessionID)).Err(); err != nil {
		return err
	}
	return r.rdb.SRem(ctx, sessionSetKey(userID), sessionID).Err()
}

func (r *sessionRepository) DeleteAll(userID string) error {
	ctx := context.Background()
	sessionIDs, err := r.rdb.SMembers(ctx, sessionSetKey(userID)).Result()
	if err != nil {
		return err
	}

	for _, id := range sessionIDs {
		if err := r.rdb.Del(ctx, sessionKey(userID, id)).Err(); err != nil {
			return err
		}
	}
	return r.rdb.Del(ctx, sessionSetKey(userID)).Err()
}
