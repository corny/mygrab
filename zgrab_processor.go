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

	// waiting jobs
	channel chan *zlib.GrabTarget

	// finished jobs
	finished chan *HostResult

	workersDone  sync.WaitGroup
	finisherDone sync.WaitGroup
}

func NewZgrabProcessor(workersCount uint) *ZgrabProcessor {
	proc := &ZgrabProcessor{}
	proc.channel = make(chan *zlib.GrabTarget, 100)
	proc.finished = make(chan *HostResult)
	proc.jobs = make(map[*zlib.GrabTarget]bool)

	proc.finisherDone.Add(1)
	proc.workersDone.Add(int(workersCount))

	// Start all workers
	for i := uint(0); i < workersCount; i++ {
		go func() {
			for target := range proc.channel {
				result := NewHostResult(*target)
				proc.finished <- &result
			}
			proc.workersDone.Done()
		}()
	}

	// Start the result saver
	go func() {
		for result := range proc.finished {
			saveHostResult(*result)
		}
		proc.finisherDone.Done()
	}()

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
		proc.channel <- target
	}
}

// Stops accepting new jobs and waits until all jobs are finished
func (proc *ZgrabProcessor) Close() {
	close(proc.channel)
	proc.workersDone.Wait()

	close(proc.finished)
	proc.finisherDone.Wait()
}
