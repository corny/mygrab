package main

import (
	"flag"
	"github.com/zmap/zgrab/zlib"
	"github.com/zmap/zgrab/ztools/zlog"
	"os"
	"runtime"
	"time"
)

var zlibConfig = &zlib.Config{}

type Decoder interface {
	DecodeNext() (interface{}, error)
}

func init() {

	flag.StringVar(&zlibConfig.EHLODomain, "ehlo", "example.com", "Send an EHLO with the specified domain (implies --smtp)")
	flag.UintVar(&zlibConfig.Senders, "senders", 100, "Number of send coroutines to use")
	flag.Parse()

	zlibConfig.ErrorLog = zlog.New(os.Stderr, "banner-grab")
	zlibConfig.Port = 25
	zlibConfig.SMTP = true
	zlibConfig.StartTLS = true
	zlibConfig.Banners = true
	zlibConfig.EHLO = true
	zlibConfig.Timeout = time.Duration(10) * time.Second

	//unboundCtx.DebugLevel(2)

	connect("dbname=survey_development host=/var/run/postgresql")
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU() + 2)

	go worker()
	Resolve("digineo.de")

	//decoder := zlib.NewGrabTargetDecoder(os.Stdin)
	//Process(decoder, *zlibConfig)
	//Resolve("digineo.de")

}
