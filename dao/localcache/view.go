package localcache

import (
	"fmt"
	"time"

	"github.com/bluele/gcache"
	priorityqueue "github.com/emirpasic/gods/queues/priorityqueue"
	"github.com/pkg/errors"
)

func IncrView(objType int, objID int64, offset int) error {
	cacheKey := getCacheKey(objType, objID)
	view, err := viewCache.Get(cacheKey)
	if err != nil {
		// key 不存在，创建
		if errors.Is(err, gcache.KeyNotFoundError) {
			viewCache.Set(cacheKey, viewObj{
				objID:     objID,
				objType:   objType,
				view:      offset,
				timeStamp: time.Now().Unix(),
			})
		} else {
			// 未知错误
			return errors.Wrap(err, "localcache:IncrView: Get")
		}
		return nil
	}
	v := view.(viewObj)
	v.view += offset
	return errors.Wrap(viewCache.Set(cacheKey, v), "localcache:IncrView: Set")
}

func SetViewCreateTime(objType int, objID, timeStamp int64) error {
	cacheKey := getCacheKey(objType, objID)
	return createTimeCache.Set(cacheKey, timeStamp)
}

func GetTopKObjectIDByViews(objType int, k int) ([]int64, error) {
	pq := priorityqueue.NewWith(cmp) // 小根堆

	// 获取所有的 view
	all := viewCache.GetALL(false)
	for _, value := range all {
		v := value.(viewObj)
		if objType != v.objType {
			continue
		}

		// TopK 问题
		if pq.Size() == k {
			t, _ := pq.Peek()
			topView := t.(viewObj)
			if v.view > topView.view {
				pq.Dequeue()
				pq.Enqueue(v)
			}
		} else {
			pq.Enqueue(v)
		}
	}

	res := make([]int64, 0, k)
	for {
		oView, ok := pq.Dequeue()
		if !ok {
			break
		}
		res = append(res, oView.(viewObj).objID)
	}

	return res, nil
}

func RemoveObjectView(objType int, objID int64) bool {
	cacheKey := getCacheKey(objType, objID)
	return viewCache.Remove(cacheKey)
}

func RemoveExpiredObjectView(targetTimeStamp int64) {
	all := viewCache.GetALL(false)
	for key, val := range all {
		cacheKey := key.(string)
		v := val.(viewObj)
		if v.timeStamp < targetTimeStamp {
			viewCache.Remove(cacheKey)
			localcache.Remove(cacheKey)
		}
	}
}

func getCacheKey(objType int, objID int64) string {
	return fmt.Sprintf("%v_%v", objType, objID)
}

type viewObj struct {
	objID     int64
	objType   int
	view      int
	timeStamp int64
}

func cmp(a, b interface{}) int {
	aAsserted := a.(viewObj)
	bAsserted := b.(viewObj)
	switch {
	case aAsserted.view > bAsserted.view:
		return 1
	case aAsserted.view < bAsserted.view:
		return -1
	default:
		return 0
	}
}
