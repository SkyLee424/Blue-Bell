package workers

import "bluebell/logger"

var done chan int		// 标记主 goroutine 即将退出
var semWorker chan int  // 看作信号量，代表当前正在运行的后台 worker 数量

const total = 12 // 后台任务的数量

func InitWorkers() {
	done = make(chan int, total)
	semWorker = make(chan int, total)
	for i := 0; i < total; i++ {
		semWorker <- 1
	}

	PersistencePostScore()

	PersistenceCommentCount(true)
	PersistenceCommentCount(false)
	PersistenceCommentCidUid(true)
	PersistenceCommentCidUid(false)
	RemoveCommentCidUidFromDB()
	RemoveCommentIndexFromRedis()
	RemoveCommentContentFromRedis()

	RefreshHotPost()
	RefreshPostHotSpot()
	RefreshCommentHotSpot()
	RemoveExpiredObjectView()
}

func Wait() {
	// 给后台任务传递消息
	for i := 0; i < total; i++ {
		done <- 1
	}
	// 阻塞等待所有后台任务退出
	for i := 0; i < total; i++ {
		<-semWorker
	}
	logger.Infof("All background workers have exited")
}
