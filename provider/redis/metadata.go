package redis

import (
	"errors"
	"time"
)

const (
	metadataKey   = `__private_provider_redis`
	preparePrefix = `prepare.`
	readyPrefix   = `ready.`
)

func checkKey(key string) (e error) {
	if key == metadataKey {
		e = errors.New(`key is reserved : ` + key)
		return
	}
	return
}

type _Metadata struct {
	Access          string
	Refresh         string
	AccessDeadline  time.Time
	RefreshDeadline time.Time
}

func newMetadata(access, refresh string, accessDuration, refreshDuration time.Duration) *_Metadata {
	at := time.Now()
	return &_Metadata{
		Access:          access,
		Refresh:         refresh,
		AccessDeadline:  at.Add(accessDuration),
		RefreshDeadline: at.Add(refreshDuration),
	}
}
func (md *_Metadata) IsExpired() bool {
	return !md.AccessDeadline.After(time.Now())
}
func (md *_Metadata) IsDeleted() bool {
	return !md.RefreshDeadline.After(time.Now())
}
