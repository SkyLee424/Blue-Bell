package localcache

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bluele/gcache"
	priorityqueue "github.com/emirpasic/gods/queues/priorityqueue"
	"github.com/pkg/errors"
)

func IncrView(objType int, objID int64, offset int) (bool, error) {
	cacheKey := getCacheKey(objType, objID)
	view, err := viewCache.Get(cacheKey)
	if err == nil { // cache hit
		if view.(int)+offset == 0 {
			viewCache.Remove(cacheKey)
		} else {
			viewCache.Set(cacheKey, view.(int)+offset)
		}
		return false, nil
	} else if errors.Is(err, gcache.KeyNotFoundError) { // cache miss
		return true, errors.Wrap(viewCache.Set(cacheKey, offset), "localcache:IncrView: Set")
	} else { // other error
		return false, errors.Wrap(err, "localcache:IncrView: Set")
	}
}

func SetViewCreateTime(objType int, objID, timeStamp int64) error {
	cacheKey := getCacheKey(objType, objID)
	return createTimeCache.Set(cacheKey, timeStamp)
}

func GetTopKObjectIDByViews(objType int, k int) ([]int64, error) {
	pq := priorityqueue.NewWith(cmp) // 小根堆

	// 获取所有的 view
	all := viewCache.GetALL(false)
	for key, value := range all {
		cacheKey := key.(string)
		view := value.(int)
		tmp := strings.Split(cacheKey, "_")
		_objType, _ := strconv.ParseInt(tmp[0], 10, 32)
		if objType != int(_objType) {
			continue
		}
		objID, _ := strconv.ParseInt(tmp[1], 10, 64)
		oView := viewObj{
			objID: objID,
			view:  view,
		}

		// TopK 问题
		if pq.Size() == k {
			t, _ := pq.Peek()
			topView := t.(viewObj)
			if view > topView.view {
				pq.Dequeue()
				pq.Enqueue(oView)
			}
		} else {
			pq.Enqueue(oView)
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

func RemoveExpiredObjectView(targetTimeStamp int64) {
	all := createTimeCache.GetALL(false)
	for k, v := range all {
		cacheKey := k.(string)
		createTime := v.(int64)
		if createTime < targetTimeStamp {
			createTimeCache.Remove(cacheKey)
		}
	}
}

func getCacheKey(objType int, objID int64) string {
	return fmt.Sprintf("%v_%v", objType, objID)
}

type viewObj struct {
	objID int64
	view  int
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
