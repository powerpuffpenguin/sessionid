# sessionid
authentication by sessionid package

support a variety of back-end storage session data:

* memory
* bolt (a local file databases)
* redis

```
m := sessionid.NewManager(
    sessionid.WithProvider(
        sessionid.NewProvider(), // memory provider
    ),
)
// Create session
session, refresh, _ := m.Create(ctx, id)
fmt.Println(`access token:`, session.Token())
fmt.Println(`refresh token:`, refresh)
// session,_ = m.Get(session.Token(), id) 

// Put key value
session.Put(ctx, sessionid.Pair{
    Key:   `uid`,
    Value: 1,
}, sessionid.Pair{
    Key:   `authorization`,
    Value: []int{1, 2, 3, 4, 5},
})

// Get value
var uid int
session.Get(ctx, `uid`, &uid)
var authorization []int
session.Get(ctx, `authorization`, &uid)
```

# Manager

Used to create a token for the session and verify the token signature

```
type Manager interface {
	// Create a session for the user
	//
	// * id uid or web-uid or mobile-uid or ...
	//
	// * pair session init key value
	Create(ctx context.Context,
		id string,
		pair ...Pair,
	) (session *Session, refresh string, e error)
	// Destroy a session by id
	Destroy(ctx context.Context, id string) error
	// Destroy a session by token
	DestroyByToken(ctx context.Context, token string) error
	// Get session from token
	Get(ctx context.Context, token string) (s *Session, e error)
	// Refresh a new access token
	Refresh(ctx context.Context, access, refresh string) (newAccess, newRefresh string, e error)
}
```

# Provider

Data storage abstraction

```
type Provider interface {
	// Create new session
	Create(ctx context.Context,
		access, refresh string, // id.sessionid.signature
		pair []PairBytes,
	) (e error)
	// Destroy a session by id
	Destroy(ctx context.Context, id string) (e error)
	// Destroy a session by token
	DestroyByToken(ctx context.Context, token string) (e error)
	// Check token status
	Check(ctx context.Context, token string) (e error)
	// Put key value for token
	Put(ctx context.Context, token string, pair []PairBytes) (e error)
	// Get key's value from token
	Get(ctx context.Context, token string, keys []string) (vals []Value, e error)
	// Keys return all token's key
	Keys(ctx context.Context, token string) (keys []string, e error)
	// Delete keys
	Delete(ctx context.Context, token string, keys []string) (e error)
	// Refresh a new access token
	Refresh(ctx context.Context, access, refresh, newAccess, newRefresh string) (e error)
	// Close provider
	Close() (e error)
}
```