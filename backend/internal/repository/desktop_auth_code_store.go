package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

type desktopAuthCodeStore struct{ rdb *redis.Client }

func NewDesktopAuthCodeStore(rdb *redis.Client) service.DesktopAuthCodeStore {
	return &desktopAuthCodeStore{rdb: rdb}
}

func (s *desktopAuthCodeStore) Store(ctx context.Context, hash string, grant *service.DesktopAuthGrant, ttl time.Duration) error {
	data, err := json.Marshal(grant)
	if err != nil {
		return err
	}
	ok, err := s.rdb.SetNX(ctx, "desktop_auth:code:"+hash, data, ttl).Result()
	if err != nil {
		return err
	}
	if !ok {
		return service.ErrServiceUnavailable
	}
	return nil
}

var consumeDesktopAuthCode = redis.NewScript(`
local value = redis.call('GET', KEYS[1])
if not value then return '' end
local grant = cjson.decode(value)
if grant.client_id ~= ARGV[1] or grant.redirect_uri ~= ARGV[2] or grant.challenge ~= ARGV[3] then return '' end
redis.call('DEL', KEYS[1])
return value
`)

func (s *desktopAuthCodeStore) Consume(ctx context.Context, hash, clientID, redirectURI, challenge string) (*service.DesktopAuthGrant, error) {
	value, err := consumeDesktopAuthCode.Run(ctx, s.rdb, []string{"desktop_auth:code:" + hash}, clientID, redirectURI, challenge).Text()
	if err != nil {
		return nil, service.ErrServiceUnavailable
	}
	if value == "" {
		return nil, service.ErrDesktopInvalidGrant
	}
	var grant service.DesktopAuthGrant
	if err := json.Unmarshal([]byte(value), &grant); err != nil {
		return nil, service.ErrServiceUnavailable
	}
	return &grant, nil
}
