package sessionid_test

import (
	"context"
	"testing"

	"github.com/powerpuffpenguin/sessionid"
	"github.com/stretchr/testify/assert"
)

func TestManager(t *testing.T) {
	m := sessionid.NewManager(
		sessionid.WithProvider(
			sessionid.NewMemoryProvider(),
		),
	)
	ctx := context.Background()
	id := `1-app`
	session, refresh, e := m.Create(ctx, id)
	assert.Nil(t, e)
	_, e = m.Get(session.Token())
	assert.Nil(t, e)
	_, _, e = m.Refresh(ctx, session.Token(), refresh)
	assert.Nil(t, e)
	_, _, e = m.Refresh(ctx, session.Token(), refresh)
	assert.Equal(t, e, sessionid.ErrTokenNotExists)

}
