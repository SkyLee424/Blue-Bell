package workers

import "sync"

var wg sync.WaitGroup

func InitWorkers()  {
	PersistencePostScore(&wg)
	
	PersistenceCommentCount(&wg, true)
	PersistenceCommentCount(&wg, false)
	PersistenceCommentCidUid(&wg, true)
	PersistenceCommentCidUid(&wg, false)
	RemoveCommentCidUidFromDB(&wg)
	RemoveCommentIndexFromRedis(&wg)
	RemoveCommentContentFromRedis(&wg)

	RefreshHotPost(&wg)
	RefreshPostHotSpot(&wg)
	RefreshCommentHotSpot(&wg)
	RemoveExpiredObjectView(&wg)
}

func Wait()  {
	wg.Wait()
}