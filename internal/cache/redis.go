package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/ldamasio/truthmetal/internal/model"
)

const canonicalTTL = 5 * time.Minute

type Cache struct {
	client *redis.Client
}

func NewCache(client *redis.Client) *Cache {
	return &Cache{client: client}
}

func (c *Cache) GetCanonical(ctx context.Context, namespace, key string) (*model.Parameter, error) {
	data, err := c.client.Get(ctx, cacheKey(namespace, key)).Bytes()
	if err != nil {
		return nil, err
	}
	p := &model.Parameter{}
	if err := json.Unmarshal(data, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (c *Cache) SetCanonical(ctx context.Context, p *model.Parameter) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, cacheKey(p.Namespace, p.Key), data, canonicalTTL).Err()
}

func (c *Cache) InvalidateCanonical(ctx context.Context, namespace, key string) error {
	return c.client.Del(ctx, cacheKey(namespace, key)).Err()
}

func cacheKey(namespace, key string) string {
	return "tm:canonical:" + namespace + ":" + key
}
