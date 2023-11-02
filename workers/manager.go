package workers

import "sync"

var wg sync.WaitGroup

func InitWorkers()  {
	PersistenceScore(&wg)
}

func Wait()  {
	wg.Wait()
}