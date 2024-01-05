package localcache

import (
	"github.com/bluele/gcache"
	"github.com/spf13/viper"
)

var localcache gcache.Cache
var viewCache gcache.Cache
var createTimeCache gcache.Cache

func InitLocalCache() {
	size := viper.GetInt("localcache.size")
	localcache = gcache.New(size).LRU().Build()
	viewCache = gcache.New(size).LRU().Build()
	createTimeCache = gcache.New(size).LRU().Build()
	statusCache = gcache.New(size).LRU().Build()
}

func GetLocalCache() gcache.Cache {
	return localcache
}

func GetViewCache() gcache.Cache {
	return viewCache	
}

func GetCreateTimeCache() gcache.Cache {
	return createTimeCache
}