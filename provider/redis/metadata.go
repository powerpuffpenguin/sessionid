package redis

import (
	"time"
)

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
func (md *_Metadata) DoRefresh(access, refresh string, accessDuration, refreshDuration time.Duration) {
	at := time.Now()
	md.Access = access
	md.Refresh = refresh
	md.AccessDeadline = at.Add(accessDuration)
	md.RefreshDeadline = at.Add(refreshDuration)
}
