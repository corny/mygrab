package main

import (
	"bufio"
	"flag"
	"github.com/zmap/zgrab/zlib"
	"github.com/zmap/zgrab/ztools/zlog"
	"os"
	"runtime"
	"time"
)

var (
	zlibConfig        = &zlib.Config{}
	dnsWorkers   uint = 10
	dnsProcessor      = NewDnsProcessor(dnsWorkers)
)

type Decoder interface {
	DecodeNext() (interface{}, error)
}

func init() {
	zlibConfig.ErrorLog = zlog.New(os.Stderr, "banner-grab")
	zlibConfig.Port = 25
	zlibConfig.Senders = 100
	zlibConfig.SMTP = true
	zlibConfig.StartTLS = true
	zlibConfig.Banners = true
	zlibConfig.EHLO = true
	zlibConfig.Timeout = time.Duration(10) * time.Second

	flag.StringVar(&zlibConfig.EHLODomain, "ehlo", "example.com", "Send an EHLO with the specified domain (implies --smtp)")
	flag.UintVar(&zlibConfig.Senders, "senders", zlibConfig.Senders, "Number of send coroutines to use")
	flag.UintVar(&dnsWorkers, "dnsWorkers", dnsWorkers, "Number of dns workers")
	flag.Parse()

	//unboundCtx.DebugLevel(2)

	connect("dbname=survey_development host=/var/run/postgresql")
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU() + 2)

	// Read stdin
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		Process(scanner.Text())
	}

	//decoder := zlib.NewGrabTargetDecoder(os.Stdin)
	//Process(decoder, *zlibConfig)
	//Resolve("digineo.de")

}
