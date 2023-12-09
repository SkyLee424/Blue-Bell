package localcache

import (
	"bluebell/models"
	"sync"
)

var hotPostCache []*models.PostDTO

var hotPostCacheLock sync.RWMutex

func init() {
	hotPostCache = make([]*models.PostDTO, 0)
}

func GetHotPostDTOList() ([]*models.PostDTO, error) {
	hotPostCacheLock.RLock() // 上读锁
	defer hotPostCacheLock.RUnlock()

	return hotPostCache, nil
}

// 参数的 hotPosts 已经按照热度排序
func SetHotPostDTO(hotPosts []*models.PostDTO) {
	hotPostCacheLock.Lock()
	defer hotPostCacheLock.Unlock()

	hotPostCache = hotPosts
}
