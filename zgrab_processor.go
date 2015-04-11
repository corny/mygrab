package main

import (
	"github.com/zmap/zgrab/zlib"
	"sync"
)

type ZgrabProcessor struct {
	// maps pending/running queries to jobs
	jobs map[*zlib.GrabTarget]bool

	// mutex for the map
	mutex sync.Mutex

	workers *WorkerPool
}

func NewZgrabProcessor(workersCount uint) *ZgrabProcessor {
	work := func(item interface{}) {
		target, _ := item.(*zlib.GrabTarget)
		result := NewHostResult(*target)
		resultProcessor.Add(&result)
	}

	proc := &ZgrabProcessor{workers: NewWorkerPool(workersCount, work)}
	proc.jobs = make(map[*zlib.GrabTarget]bool)

	return proc
}

// Creates a new job
func (proc *ZgrabProcessor) NewJob(target *zlib.GrabTarget) {
	var exist bool

	proc.mutex.Lock()

	// Is the same target already running?
	if _, exist = proc.jobs[target]; !exist {
		proc.jobs[target] = true
	}
	proc.mutex.Unlock()

	if !exist {
		proc.workers.Add(target)
	}
}

// Stops accepting new jobs and waits until all jobs are finished
func (proc *ZgrabProcessor) Close() {
	proc.workers.Close()
}
