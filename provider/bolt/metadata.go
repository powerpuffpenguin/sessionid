package bolt

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"sync"
	"time"

	"github.com/boltdb/bolt"
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

func (md *_Metadata) IsExpired() bool {
	return !md.accessDeadline.After(time.Now())
}
func (md *_Metadata) IsDeleted() bool {
	return !md.refreshDeadline.After(time.Now())
}
func (p *Provider) getMetadata(store *bolt.Bucket, key []byte) (bucket *bolt.Bucket, md *_Metadata, e error) {
	bucket = store.Bucket(bucketMetadata)
	if bucket == nil {
		return
	}
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
func (p *Provider) deleteData(store *bolt.Bucket, key []byte) (e error) {
	bucket := store.Bucket(bucketData)
	if bucket == nil {
		return
	}
	e = bucket.Delete(key)
	return
}
func (p *Provider) deleteIDS(store *bolt.Bucket, id []byte, access string) (e error) {
	bucket := store.Bucket(bucketIDS)
	if bucket == nil {
		return
	}
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
		if str != access {
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
func (p *Provider) deleteLRU(store *bolt.Bucket, id uint64) (e error) {
	bucket := store.Bucket(bucketLRU)
	if bucket == nil {
		return
	}
	e = bucket.Delete(toBytes(id))
	return
}
func (p *Provider) decrement(store *bolt.Bucket) (e error) {
	val := store.Get(bucketCount)
	if val == nil {
		return
	}
	num := toUint64(val)
	if num > 0 {
		num--
	}
	e = store.Put(bucketCount, toBytes(num))
	return
}
