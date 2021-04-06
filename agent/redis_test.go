package agent_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/powerpuffpenguin/sessionid/agent"
	"github.com/powerpuffpenguin/sessionid/cryptoer"
	"github.com/stretchr/testify/assert"
)

type redisElement struct {
	data  string
	token string
}

func TestRedis(t *testing.T) {
	method := cryptoer.GetSigningMethod(`HMD5`)
	assert.NotNil(t, method)
	client := redis.NewClient((&redis.Options{
		Addr:     `192.168.251.4:32609`,
		Password: ``,
		DB:       0,
	}))
	a := agent.NewRedisAgent(
		client,
		agent.WithSigningKey([]byte(`cerberus is an idea`)),
	)
	keys := make(map[string]*memoryElement)
	count := 10000
	for i := 0; i < count; i++ {
		id := strconv.Itoa(i)
		keys[id] = &memoryElement{
			data: `data-` + id,
		}
	}
	ctx := context.Background()
	expiry := time.Second * 5
	for key, ele := range keys {
		token, e := a.Create(ctx, key, ele.data, expiry)
		assert.Nil(t, e)
		ele.token = token
	}
	last := time.Now()
	for key, ele := range keys {
		id, userdata, exists, e := a.Get(ctx, ele.token, 0)
		assert.Nil(t, e)
		assert.True(t, exists)
		assert.Equal(t, key, id)
		assert.Equal(t, ele.data, userdata)
	}
	duration := time.Now().Sub(last)
	if duration < expiry {
		time.Sleep(expiry - duration + time.Second)
	}
	for _, ele := range keys {
		_, _, exists, e := a.Get(ctx, ele.token, 0)
		assert.Nil(t, e)
		assert.False(t, exists)
	}
}
