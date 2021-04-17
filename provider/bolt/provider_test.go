package bolt

import (
	"context"
	"testing"
	"time"

	"github.com/powerpuffpenguin/sessionid"
	"github.com/powerpuffpenguin/sessionid/cryptoer"
	"github.com/stretchr/testify/assert"
)

type testProvider struct {
	t *testing.T
}

func (ts *testProvider) test() {
	ts.testNormal()
	ts.testClear()
	ts.testLRU()
}

func (ts *testProvider) testLRU() {
	t := ts.t
	p, e := New(
		WithFilename(`test.db`),
		WithMaxSize(3),
		WithClear(0),
	)
	assert.Nil(t, e)
	defer p.Close()
	assert.Nil(t, p.ticker)
	assert.Nil(t, p.Clear())

	method := cryptoer.SigningMethodHMD5
	key := []byte(`cerberus is an idea`)

	ctx := context.Background()
	var tokens []string
	for i := 0; i < 100; i++ {
		access, refresh, e := sessionid.CreateToken(method, key, "1-web")
		assert.Nil(t, e)
		assert.Nil(t, p.Create(ctx, access, refresh, nil))
		tokens = append(tokens, access)
		if i < 3 {
			count, e := p.debugCount()
			assert.Nil(t, e)
			assert.Equal(t, count, i+1)
			continue
		}
		count, e := p.debugCount()
		assert.Nil(t, e)
		assert.Equal(t, count, 3)

		// token := tokens[i-2]
		// tv := p.lru.Front().Value.(*tokenValue)
		// assert.Equal(t, token, tv.access)
	}
}
func (ts *testProvider) testClear() {
	const duration = time.Millisecond * 200
	t := ts.t
	p, e := New(
		WithFilename(`test.db`),
		WithClear(duration*3/2),
		WithAccess(duration),
		WithRefresh(duration*2),
	)
	assert.Nil(t, e)
	defer p.Close()
	assert.NotNil(t, p.ticker)
	assert.Nil(t, p.Clear())
	method := cryptoer.SigningMethodHMD5
	key := []byte(`cerberus is an idea`)
	access, refresh, e := sessionid.CreateToken(method, key, "1-web")
	assert.Nil(t, e)

	// not exists
	ts.onError(p, access, sessionid.ErrTokenNotExists)
	// create
	ctx := context.Background()
	assert.Nil(t, p.Create(ctx, access, refresh, nil))
	count, e := p.debugCount()
	assert.Nil(t, e)
	assert.Equal(t, count, 1)
	assert.Nil(t, p.Check(ctx, access))

	// expired
	time.Sleep(duration)
	ts.onError(p, access, sessionid.ErrTokenExpired)
	assert.Equal(t, count, 1)

	time.Sleep(duration / 2)
	accessApp, refreshApp, e := sessionid.CreateToken(method, key, "1-app")
	assert.Nil(t, e)
	assert.Nil(t, p.Create(ctx, accessApp, refreshApp, nil))

	// deleted
	time.Sleep(duration * 3 / 2)
	count, e = p.debugCount()
	assert.Nil(t, e)
	assert.Equal(t, count, 1)
	ts.onError(p, access, sessionid.ErrTokenNotExists)
}
func (ts *testProvider) testNormal() {
	const duration = time.Millisecond * 200
	t := ts.t
	p, e := New(
		WithFilename(`test.db`),
		WithClear(0),
		WithAccess(duration),
		WithRefresh(duration*2),
	)
	assert.Nil(t, e)
	defer p.Close()
	assert.Nil(t, p.ticker)
	assert.Nil(t, p.Clear())

	method := cryptoer.SigningMethodHMD5
	key := []byte(`cerberus is an idea`)
	access, refresh, e := sessionid.CreateToken(method, key, "1-web")
	assert.Nil(t, e)

	// not exists
	ts.onError(p, access, sessionid.ErrTokenNotExists)
	// create
	ctx := context.Background()
	assert.Nil(t, p.Create(ctx, access, refresh, nil))
	count, e := p.debugCount()
	assert.Nil(t, e)
	assert.Equal(t, count, 1)
	assert.Nil(t, p.Check(ctx, access))
	// put get
	pv := []sessionid.PairBytes{{
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
	time.Sleep(duration)
	ts.onError(p, access, sessionid.ErrTokenExpired)
	count, e = p.debugCount()
	assert.Nil(t, e)
	assert.Equal(t, count, 1)

	// deleted
	time.Sleep(duration)
	ts.onError(p, access, sessionid.ErrTokenNotExists)
	count, e = p.debugCount()
	assert.Nil(t, e)
	assert.Equal(t, count, 0)

	// refresh
	{
		assert.Equal(t, p.Refresh(ctx, access, refresh, access, refresh), sessionid.ErrTokenNotExists)
		assert.Nil(t, p.Create(ctx, access, refresh, nil))
		count, e = p.debugCount()
		assert.Nil(t, e)
		assert.Equal(t, count, 1)
		assert.Nil(t, p.Check(ctx, access))
		// expired
		time.Sleep(duration)
		ts.onError(p, access, sessionid.ErrTokenExpired)
		count, e = p.debugCount()
		assert.Nil(t, e)
		assert.Equal(t, count, 1)

		assert.Nil(t, p.Refresh(ctx, access, refresh, access, refresh))
		assert.Nil(t, p.Check(ctx, access))
		time.Sleep(duration)
		ts.onError(p, access, sessionid.ErrTokenExpired)
		time.Sleep(duration)
		ts.onError(p, access, sessionid.ErrTokenNotExists)
	}
	// close
	assert.Nil(t, p.Close())
	ts.onError(p, access, sessionid.ErrProviderClosed)
	e = p.Close()
	assert.Equal(t, sessionid.ErrProviderClosed, e)
}
func (ts *testProvider) onError(p *Provider, access string, target error) {
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
func TestProvider(t *testing.T) {
	ts := testProvider{
		t: t,
	}
	ts.test()
}
