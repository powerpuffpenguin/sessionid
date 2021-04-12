package sessionid

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testMemoryProvider struct {
	t *testing.T
}

func (ts *testMemoryProvider) test() {
	t := ts.t
	p := NewMemoryProvider(
		WithProviderClear(0),
		WithProviderAccess(time.Millisecond*10),
		WithProviderRefresh(time.Millisecond*20),
	)
	assert.Nil(t, p.ticker)
	m := NewManager()

	access, refresh, e := m.create("1-web")
	assert.Nil(t, e)

	// not exists
	ts.onError(p, access, ErrTokenNotExists)
	// create
	ctx := context.Background()
	assert.Nil(t, p.Create(ctx, access, refresh, nil))
	assert.Equal(t, len(p.tokens), 1)
	assert.Nil(t, p.Check(ctx, access))
	// expired
	time.Sleep(time.Millisecond * 10)
	ts.onError(p, access, ErrTokenExpired)
	assert.Equal(t, len(p.tokens), 1)

	// put get

	// deleted
	time.Sleep(time.Millisecond * 10)
	ts.onError(p, access, ErrTokenNotExists)
	assert.Equal(t, len(p.tokens), 0)

	// close
	assert.Nil(t, p.Close())
	ts.onError(p, access, ErrProviderClosed)
	e = p.Close()
	assert.Equal(t, ErrProviderClosed, e)
}
func (ts *testMemoryProvider) onError(p *MemoryProvider, access string, target error) {
	t := ts.t
	ctx := context.Background()
	e := p.Check(ctx, access)
	assert.Equal(t, target, e)
	e = p.Set(ctx, access, nil)
	assert.Equal(t, target, e)
	_, e = p.Get(ctx, access, nil)
	assert.Equal(t, target, e)
	e = p.Set(ctx, access, nil)
	assert.Equal(t, target, e)
	_, e = p.Keys(ctx, access)
	assert.Equal(t, target, e)
	e = p.Delete(ctx, access, nil)
	assert.Equal(t, target, e)
}
func TestMemoryProvider(t *testing.T) {
	ts := testMemoryProvider{
		t: t,
	}
	ts.test()
}
