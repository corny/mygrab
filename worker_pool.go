package main

import (
	"sync"
)

// The worker function receives any object and returns nothing
type WorkerFunc func(interface{})

type WorkerPool struct {
	// channel for pending jobs
	channel chan interface{}

	// Number of processed jobs
	processed uint64

	// WaitGroup to wait until all workers are done
	wg sync.WaitGroup

	// The function that does the work
	work WorkerFunc

	// Size of the worker pool
	maxWorkers     uint
	currentWorkers uint
}

func NewWorkerPool(maxWorkers uint, work WorkerFunc) *WorkerPool {
	proc := &WorkerPool{work: work}
	proc.channel = make(chan interface{}, 100)
	proc.maxWorkers = maxWorkers
	return proc
}

func (proc *WorkerPool) worker() {
	for obj := range proc.channel {
		proc.work(obj)
		proc.processed += 1 // not atomic
	}
	proc.wg.Done()
}

// Adds a new object to the channel
func (proc *WorkerPool) Add(obj interface{}) {
	if len(proc.channel) > 0 && proc.maxWorkers > proc.currentWorkers {
		// Start another worker
		proc.currentWorkers++ // not atomic, not crucial
		proc.wg.Add(1)
		go proc.worker()
	}
	proc.channel <- obj
}

// Stops accepting new jobs and waits until all workers are finished
func (proc *WorkerPool) Close() {
	close(proc.channel)
	proc.wg.Wait()
}
