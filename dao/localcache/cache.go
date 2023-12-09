package localcache

import (
	"bluebell/models"
	"sync"
)

var hotPostCache map[int64]*models.PostDTO

var hotPostCacheLock sync.RWMutex

func init() {
	hotPostCache = make(map[int64]*models.PostDTO)
}

func GetHotPostDTOList() ([]*models.PostDTO, error) {
	hotPostCacheLock.RLock() // 上读锁
	defer hotPostCacheLock.RUnlock()

	list := make([]*models.PostDTO, 0, len(hotPostCache))
	for _, postDTO := range hotPostCache {
		list = append(list, postDTO)
	}

	return list, nil
}

func SetHotPostDTO(hotPosts []*models.PostDTO) {
	newHotPostCache := make(map[int64]*models.PostDTO)
	for _, hotPost := range hotPosts {
		newHotPostCache[hotPost.PostID] = hotPost
	}

	hotPostCacheLock.Lock()
	defer hotPostCacheLock.Unlock()

	hotPostCache = newHotPostCache
}
