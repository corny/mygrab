package main

import (
	"log"
	"sync"
)

type WorkerFunc func(interface{})

type WorkerPool struct {
	channel chan interface{}
	wg      sync.WaitGroup
	work    WorkerFunc
}

func NewWorkerPool(workersCount uint, work WorkerFunc) *WorkerPool {
	proc := &WorkerPool{work: work}
	proc.channel = make(chan interface{}, 100)
	proc.wg.Add(int(workersCount))

	// Start all workers
	for i := uint(0); i < workersCount; i++ {
		go proc.worker()
	}

	return proc
}

func (proc *WorkerPool) worker() {
	for obj := range proc.channel {
		log.Println("work:", obj)
		proc.work(obj)
	}
	proc.wg.Done()
}

// Adds a new object to the channel
func (proc *WorkerPool) Add(obj interface{}) {

	log.Printf("add: %p %s", &obj, obj)
	proc.channel <- obj
}

// Stops accepting new jobs and waits until all workers are finished
func (proc *WorkerPool) Close() {
	close(proc.channel)
	proc.wg.Wait()
}
