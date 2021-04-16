package sessionid_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/powerpuffpenguin/sessionid"
	"github.com/stretchr/testify/assert"
)

func TestSession(t *testing.T) {
	m := sessionid.NewManager(
		sessionid.WithProvider(
			sessionid.NewProvider(),
		),
	)
	ctx := context.Background()
	id := `1-app`
	session, _, e := m.Create(ctx, id)
	assert.Nil(t, e)
	e = session.Put(ctx, sessionid.Pair{
		Key:   `id`,
		Value: 1,
	}, sessionid.Pair{
		Key:   `authorization`,
		Value: []int{1, 2, 3, 4, 5},
	})
	assert.Nil(t, e)
	var uid int
	e = session.Get(ctx, `abc`, &uid)
	assert.Equal(t, e, sessionid.ErrKeyNotExists)
	e = session.Get(ctx, `id`, &uid)
	assert.Nil(t, e)
	assert.Equal(t, uid, 1)
	e = session.Get(ctx, `id`, &uid)
	assert.Nil(t, e)
	assert.Equal(t, uid, 1)

	// Prepare
	e = session.Prepare(ctx, `id`, `authorization`)
	assert.Nil(t, e)
	var authorization []int
	e = session.Get(ctx, `authorization`, &authorization)
	assert.Nil(t, e)
	assert.Equal(t, fmt.Sprint(authorization), `[1 2 3 4 5]`)
	authorization[0] = 1
	e = session.Get(ctx, `authorization`, &authorization)
	assert.Nil(t, e)
	assert.Equal(t, fmt.Sprint(authorization), `[1 2 3 4 5]`)

	// Put
	e = session.Put(ctx, sessionid.Pair{
		Key:   `id`,
		Value: 2,
	})
	assert.Nil(t, e)
	e = session.Get(ctx, `id`, &uid)
	assert.Nil(t, e)
	assert.Equal(t, uid, 2)

	var i int32
	e = session.Get(ctx, `id`, &i)
	assert.Nil(t, e)
	assert.Equal(t, i, int32(2))
}
