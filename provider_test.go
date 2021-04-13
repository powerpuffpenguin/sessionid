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
	ts.testNormal()
	ts.testClear()
	ts.testLRU()
}
func (ts *testMemoryProvider) testLRU() {
	t := ts.t
	p := NewMemoryProvider(
		WithProviderMaxSize(3),
	)

	m := NewManager()
	ctx := context.Background()
	var tokens []string
	for i := 0; i < 1000; i++ {
		access, refresh, e := m.create("1-web")
		assert.Nil(t, e)
		assert.Nil(t, p.Create(ctx, access, refresh, nil))
		tokens = append(tokens, access)
		if i < 3 {
			assert.Equal(t, len(p.tokens), i+1)
			continue
		}
		assert.Equal(t, len(p.tokens), 3)
		token := tokens[i-2]
		tv := p.lru.Front().Value.(*tokenValue)
		assert.Equal(t, token, tv.access)
	}
}
func (ts *testMemoryProvider) testClear() {
	t := ts.t
	p := NewMemoryProvider(
		WithProviderClear(30),
		WithProviderAccess(time.Millisecond*10),
		WithProviderRefresh(time.Millisecond*20),
	)
	assert.NotNil(t, p.ticker)
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

	time.Sleep(time.Millisecond * 5)
	accessApp, refreshApp, e := m.create("1-app")
	assert.Nil(t, e)
	assert.Nil(t, p.Create(ctx, accessApp, refreshApp, nil))

	// deleted
	time.Sleep(time.Millisecond * 15)
	assert.Equal(t, len(p.tokens), 1)
	ts.onError(p, access, ErrTokenNotExists)

}
func (ts *testMemoryProvider) testNormal() {
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
	// put get
	pv := []PairBytes{{
		Key:   `k`,
		Value: []byte("ok"),
	}}
	e = p.Put(ctx, access, pv)
	assert.Nil(t, e)
	vals, e := p.Get(ctx, access, []string{`k`})
	assert.Nil(t, e)
	assert.Equal(t, len(vals), 1)
	assert.True(t, vals[0].Exists)
	assert.True(t, string(vals[0].Bytes) == `ok`)
	// expired
	time.Sleep(time.Millisecond * 10)
	ts.onError(p, access, ErrTokenExpired)
	assert.Equal(t, len(p.tokens), 1)

	// deleted
	time.Sleep(time.Millisecond * 10)
	ts.onError(p, access, ErrTokenNotExists)
	assert.Equal(t, len(p.tokens), 0)

	// refresh
	{
		assert.Equal(t, p.Refresh(ctx, access, refresh, access, refresh), ErrTokenNotExists)
		assert.Nil(t, p.Create(ctx, access, refresh, nil))
		assert.Equal(t, len(p.tokens), 1)
		assert.Nil(t, p.Check(ctx, access))
		// expired
		time.Sleep(time.Millisecond * 10)
		ts.onError(p, access, ErrTokenExpired)
		assert.Equal(t, len(p.tokens), 1)

		assert.Nil(t, p.Refresh(ctx, access, refresh, access, refresh))
		assert.Nil(t, p.Check(ctx, access))
		time.Sleep(time.Millisecond * 10)
		ts.onError(p, access, ErrTokenExpired)
		time.Sleep(time.Millisecond * 10)
		ts.onError(p, access, ErrTokenNotExists)
	}
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
	e = p.Put(ctx, access, nil)
	assert.Equal(t, target, e)
	_, e = p.Get(ctx, access, nil)
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
