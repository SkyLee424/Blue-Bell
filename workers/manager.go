package workers

import "sync"

var wg sync.WaitGroup

func InitWorkers()  {
	PersistencePostScore(&wg)
	RefreshHotPost(&wg)
	RefreshPostHotSpot(&wg)
	
	PersistenceCommentCount(&wg, true)
	PersistenceCommentCount(&wg, false)
	PersistenceCommentCidUid(&wg, true)
	PersistenceCommentCidUid(&wg, false)
	RemoveCommentCidUidFromDB(&wg)
	RemoveCommentIndexFromRedis(&wg)
	RemoveCommentContentFromRedis(&wg)
}

func Wait()  {
	wg.Wait()
}