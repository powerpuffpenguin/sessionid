package bolt

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/powerpuffpenguin/sessionid"
)

var poolbuffer = sync.Pool{
	New: func() interface{} {
		return &bytes.Buffer{}
	},
}

func getBuffer() *bytes.Buffer {
	return poolbuffer.Get().(*bytes.Buffer)
}
func putBuffer(buf *bytes.Buffer) {
	buf.Reset()
	poolbuffer.Put(buf)
}
func toBytes(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}
func toUint64(b []byte) uint64 {
	return binary.BigEndian.Uint64(b)
}

type _Metadata struct {
	id              string
	access          string
	refresh         string
	accessDeadline  time.Time
	refreshDeadline time.Time

	lru uint64
}

func newMetadata(id, access, refresh string, accessDuration, refreshDuration time.Duration) *_Metadata {
	at := time.Now()
	return &_Metadata{
		id:              id,
		access:          access,
		refresh:         refresh,
		accessDeadline:  at.Add(accessDuration),
		refreshDeadline: at.Add(refreshDuration),
	}
}
func (md *_Metadata) IsExpired() bool {
	return !md.accessDeadline.After(time.Now())
}
func (md *_Metadata) IsDeleted() bool {
	return !md.refreshDeadline.After(time.Now())
}
func (md *_Metadata) Refresh(access, refresh string, accessDuration, refreshDuration time.Duration) {
	at := time.Now()
	md.access = access
	md.refresh = refresh
	md.accessDeadline = at.Add(accessDuration)
	md.refreshDeadline = at.Add(refreshDuration)
}
func (p *Provider) getMetadata(bucket *bolt.Bucket, key []byte) (md *_Metadata, e error) {
	b := bucket.Get(key)
	if b == nil {
		return
	}
	var tmp _Metadata
	dec := gob.NewDecoder(bytes.NewBuffer(b))
	e = dec.Decode(&tmp)
	if e != nil {
		return
	}
	md = &tmp
	return
}
func (p *Provider) putMetadata(bucket *bolt.Bucket, key []byte, md *_Metadata) (e error) {
	buf := getBuffer()
	e = gob.NewEncoder(buf).Encode(md)
	if e != nil {
		putBuffer(buf)
		return
	}
	e = bucket.Put(key, buf.Bytes())
	putBuffer(buf)
	if e != nil {
		return
	}
	return
}
func (p *Provider) deleteIDS(bucket *bolt.Bucket, id []byte, token string) (e error) {
	b := bucket.Get(id)
	if b == nil {
		return
	}
	var strs []string
	dec := gob.NewDecoder(bytes.NewBuffer(b))
	e = dec.Decode(&strs)
	if e != nil {
		return
	}
	for i, str := range strs {
		if str != token {
			continue
		}
		if len(strs) == 1 {
			e = bucket.Delete(id)
		} else {
			copy(strs[i:], strs[i+1:])
			strs = strs[:len(strs)-1]
			buf := getBuffer()
			e = gob.NewEncoder(buf).Encode(strs)
			if e != nil {
				putBuffer(buf)
				break
			}
			e = bucket.Put(id, buf.Bytes())
			putBuffer(buf)
			if e != nil {
				break
			}
		}
		break
	}
	return
}
func (p *Provider) appendIDS(bucket *bolt.Bucket, id []byte, token string) (e error) {
	b := bucket.Get(id)
	var strs []string
	if b != nil {
		dec := gob.NewDecoder(bytes.NewBuffer(b))
		e = dec.Decode(&strs)
		if e != nil {
			return
		}
	}
	strs = append(strs, token)

	buf := getBuffer()
	e = gob.NewEncoder(buf).Encode(strs)
	if e != nil {
		putBuffer(buf)
		return
	}

	e = bucket.Put(id, buf.Bytes())
	putBuffer(buf)
	if e != nil {
		return
	}
	return
}
func (p *Provider) putLRU(bucket *bolt.Bucket, val []byte) (k uint64, e error) {
	k, e = bucket.NextSequence()
	if e != nil {
		return
	}
	e = bucket.Put(toBytes(k), val)
	return
}
func (p *Provider) putData(bucket *bolt.Bucket, key []byte, pairs []sessionid.PairBytes) (e error) {
	bucket, e = bucket.CreateBucketIfNotExists(key)
	if e != nil {
		return
	}
	for _, pair := range pairs {
		e = bucket.Put([]byte(pair.Key), pair.Value)
		if e != nil {
			return
		}
	}
	return
}
func (p *Provider) getData(bucket *bolt.Bucket, token string, key []string) (pairs []sessionid.Value, e error) {
	if len(key) == 0 {
		return
	}
	pairs = make([]sessionid.Value, len(key))
	bucket = bucket.Bucket([]byte(token))
	if bucket == nil {
		return
	}
	for i, k := range key {
		b := bucket.Get([]byte(k))
		if b != nil {
			pairs[i] = sessionid.Value{
				Bytes:  b,
				Exists: true,
			}
		}
	}
	return
}
func (p *Provider) getKeys(bucket *bolt.Bucket, token string) (keys []string, e error) {
	bucket = bucket.Bucket([]byte(token))
	if bucket == nil {
		return
	}
	cursor := bucket.Cursor()
	for k, _ := cursor.First(); k != nil; k, _ = cursor.Next() {
		keys = append(keys, string(k))
	}
	return
}
func (p *Provider) getPairs(bucket *bolt.Bucket, token string) (pairs []sessionid.PairBytes, e error) {
	bucket = bucket.Bucket([]byte(token))
	if bucket == nil {
		return
	}
	cursor := bucket.Cursor()
	for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
		pairs = append(pairs, sessionid.PairBytes{
			Key:   string(k),
			Value: v,
		})
	}
	return
}

func (p *Provider) deleteKeys(bucket *bolt.Bucket, token string, keys []string) (e error) {
	bucket = bucket.Bucket([]byte(token))
	if bucket == nil {
		return
	}
	for _, key := range keys {
		e = bucket.Delete([]byte(key))
		if e != nil {
			return
		}
	}
	return
}

func (p *Provider) decrement(store *bolt.Bucket) (e error) {
	val := store.Get(keyCount)
	if val == nil {
		return
	}
	num := toUint64(val)
	if num > 0 {
		num--
	}
	e = store.Put(keyCount, toBytes(num))
	return
}
func (p *Provider) increment(store *bolt.Bucket) (e error) {
	val := store.Get(keyCount)
	if val == nil {
		return
	}
	num := toUint64(val)
	num++
	e = store.Put(keyCount, toBytes(num))
	return
}
func (p *Provider) count(store *bolt.Bucket) (val uint64) {
	b := store.Get(keyCount)
	if b == nil {
		return
	}
	val = toUint64(b)
	return
}
