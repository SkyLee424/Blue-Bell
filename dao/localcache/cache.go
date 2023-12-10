package localcache

import (
	"github.com/bluele/gcache"
	"github.com/spf13/viper"
)

var localcache gcache.Cache

func InitLocalCache() {
	size := viper.GetInt("localcache.size")
	localcache = gcache.New(size).LRU().Build()
}

func GetLocalCache() gcache.Cache {
	return localcache
}
