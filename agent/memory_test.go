package agent_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/powerpuffpenguin/sessionid/agent"
	"github.com/powerpuffpenguin/sessionid/cryptoer"
	"github.com/stretchr/testify/assert"
)

type memoryElement struct {
	data  string
	token string
}

func TestMemory(t *testing.T) {
	method := cryptoer.GetSigningMethod(`HMD5`)
	assert.NotNil(t, method)
	a := agent.NewMemoryAgent(
		agent.WithWheel(time.Millisecond*50, 20),
		agent.WithSigningMethod(method),
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
	for key, ele := range keys {
		token, e := a.Create(ctx, key, ele.data, time.Millisecond*200)
		assert.Nil(t, e)
		ele.token = token
	}
	for key, ele := range keys {
		id, userdata, exists, e := a.Get(ctx, ele.token)
		assert.Nil(t, e)
		assert.True(t, exists)
		assert.Equal(t, key, id)
		assert.Equal(t, ele.data, userdata)
	}
	time.Sleep(time.Millisecond * 500)
	for _, ele := range keys {
		_, _, exists, e := a.Get(ctx, ele.token)
		assert.Nil(t, e)
		assert.False(t, exists)
	}
}
