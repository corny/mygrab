package main

import (
	"github.com/zmap/zgrab/zlib"
	"github.com/zmap/zgrab/ztools/zlog"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

var zlibConfig = &zlib.Config{}

type Decoder interface {
	DecodeNext() (interface{}, error)
}

func init() {
	zlibConfig.ErrorLog = zlog.New(os.Stderr, "banner-grab")
	zlibConfig.Port = 25
	zlibConfig.SMTP = true
	zlibConfig.StartTLS = true
	zlibConfig.Banners = true
	zlibConfig.EHLO = true
	zlibConfig.EHLODomain = "example.com"
	zlibConfig.Senders = 100
	zlibConfig.Timeout = time.Duration(10) * time.Second
}

func Process(in Decoder, config zlib.Config) {
	workers := config.Senders
	processQueue := make(chan zlib.GrabTarget, workers*4)
	outputQueue := make(chan HostResult, workers*4)

	w := zlib.NewGrabWorker(&config)

	// Create wait groups
	var workerDone sync.WaitGroup
	var outputDone sync.WaitGroup
	workerDone.Add(int(workers))
	outputDone.Add(1)

	// Start the output handler
	go func() {
		for result := range outputQueue {
			saveOutput(result)
		}
		outputDone.Done()
	}()

	// Start all the workers
	for i := uint(0); i < workers; i++ {
		go func() {
			for obj := range processQueue {
				outputQueue <- NewHostResult(obj)
			}
			workerDone.Done()
		}()
	}

	// Read the input, send to workers
	for {
		obj, err := in.DecodeNext()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}

		target, ok := obj.(zlib.GrabTarget)
		if !ok {
			panic("unable to cast")
		}
		processQueue <- target
	}
	close(processQueue)
	workerDone.Wait()
	close(outputQueue)
	outputDone.Wait()
	w.Done()
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	connect("dbname=survey_development host=/var/run/postgresql")

	decoder := zlib.NewGrabTargetDecoder(os.Stdin)

	Process(decoder, *zlibConfig)
}
