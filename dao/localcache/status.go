package localcache

import (
	"github.com/bluele/gcache"
)

var statusCache gcache.Cache

const (
	StatusSuccess = iota + 1
	StatusFailed
)

const (
	ObjComment = iota + 1
)

// func GetStatusCache() gcache.Cache {
// 	return statusCache
// }

func SetStatus(key string, status int)  {
	statusCache.Set(key, status)
}

func GetStatus(key string) (int, bool) {
	status, err := statusCache.Get(key)
	if err != nil {
		return StatusFailed, false
	}
	s, _ := status.(int)
	return s, true
}

func RemoveStatus(key string) bool {
	return statusCache.Remove(key)
}