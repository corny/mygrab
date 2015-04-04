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

type Decoder interface {
	DecodeNext() (interface{}, error)
}

func Process(in Decoder, config zlib.Config) {
	workers := config.Senders
	processQueue := make(chan zlib.GrabTarget, workers*4)
	outputQueue := make(chan *zlib.Grab, workers*4)

	w := zlib.NewGrabWorker(&config)

	// Create wait groups
	var workerDone sync.WaitGroup
	var outputDone sync.WaitGroup
	workerDone.Add(int(workers))
	outputDone.Add(1)

	// Start the output handler
	go func() {
		for out := range outputQueue {
			saveOutput(*out)
		}
		outputDone.Done()
	}()

	// Start all the workers
	for i := uint(0); i < workers; i++ {
		go func() {
			for obj := range processQueue {
				outputQueue <- zlib.GrabBanner(&config, &obj)
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

	config := zlib.Config{}
	config.ErrorLog = zlog.New(os.Stderr, "banner-grab")
	config.Port = 25
	config.SMTP = true
	config.StartTLS = true
	config.Banners = true
	config.EHLO = true
	config.EHLODomain = "example.com"
	config.Senders = 100
	config.Timeout = time.Duration(10) * time.Second

	connect("dbname=survey_development host=/var/run/postgresql")

	decoder := zlib.NewGrabTargetDecoder(os.Stdin)

	Process(decoder, config)
}
