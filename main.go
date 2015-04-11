package main

import (
	"bufio"
	"flag"
	"github.com/zmap/zgrab/zlib"
	"github.com/zmap/zgrab/ztools/zlog"
	_ "log"
	"os"
	"runtime"
	"time"
)

var (
	zlibConfig           = &zlib.Config{}
	dnsWorkers      uint = 50
	zgrabWorkers    uint = 50
	domainWorkers   uint = 50
	dnsProcessor         = NewDnsProcessor(dnsWorkers)
	zgrabProcessor       = NewZgrabProcessor(zgrabWorkers)
	domainProcessor      = NewDomainProcessor(domainWorkers)
	resultProcessor      = NewResultProcessor(1)
)

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
	zlibConfig.Timeout = time.Duration(10) * time.Second

	flag.StringVar(&zlibConfig.EHLODomain, "ehlo", "example.com", "Send an EHLO with the specified domain (implies --smtp)")
	flag.UintVar(&zgrabWorkers, "zgrabWorkers", zgrabWorkers, "Number of send coroutines to use")
	flag.UintVar(&dnsWorkers, "dnsWorkers", dnsWorkers, "Number of dns workers")
	flag.Parse()

	//unboundCtx.DebugLevel(2)

	connect("dbname=survey_development host=/var/run/postgresql")
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Read stdin
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		domainProcessor.Add(scanner.Text())
	}

	//domainProcessor.Close()
	domainProcessor.Close()
	dnsProcessor.Close()
	zgrabProcessor.Close()
	resultProcessor.Close()

}
