package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/zmap/zgrab/zlib"
	"github.com/zmap/zgrab/ztools/zlog"
	"log"
	"os"
	"runtime"
	"time"
)

var (
	socketPath      string
	zlibConfig           = &zlib.Config{}
	dnsResolver          = "8.8.8.8:53"
	dnsWorkers      uint = 500
	dnsTimeout      uint = 10 // seconds
	zgrabWorkers    uint = 500
	mxWorkers       uint = 250 // should at least as many as dnsWorkers
	domainWorkers   uint = 250 // should at least as many as dnsWorkers
	resultWorkers   uint = 2
	unboundDebug    uint = 0
	dbname          string
	dnsProcessor    *DnsProcessor
	zgrabProcessor  *ZgrabProcessor
	domainProcessor *DomainProcessor
	mxProcessor     *MxProcessor
	resultProcessor *ResultProcessor
	nsUpdater       *NsUpdater
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
	zlibConfig.EHLODomain, _ = os.Hostname()
}

func main() {
	var singleWorker bool

	flag.StringVar(&zlibConfig.EHLODomain, "ehlo", zlibConfig.EHLODomain, "Send an EHLO with the specified domain (implies --smtp)")
	flag.StringVar(&dnsResolver, "dnsResolver", dnsResolver, "DNS resolver address")
	flag.UintVar(&dnsTimeout, "dnsTimeout", dnsTimeout, "DNS timeout in seconds")

	flag.StringVar(&nsupdateKey, "nsupdateKey", "", "path to nsupdate key")
	flag.UintVar(&nsupdateTTL, "nsupdateTTL", nsupdateTTL, "TTL for DNS entries")
	flag.StringVar(&nsupdateServer, "nsupdateServer", nsupdateServer, "nsupdate server")

	flag.StringVar(&socketPath, "socket", "", "Read from a socket instead of stdin")
	flag.UintVar(&dnsWorkers, "dnsWorkers", dnsWorkers, "Number of dns workers")
	flag.UintVar(&mxWorkers, "mxWorkers", mxWorkers, "Number of mx workers")
	flag.UintVar(&zgrabWorkers, "zgrabWorkers", zgrabWorkers, "Number of zgrab workers")
	flag.UintVar(&domainWorkers, "domainWorkers", domainWorkers, "Number of dns workers")
	flag.UintVar(&resultWorkers, "resultWorkers", resultWorkers, "Number of result workers that store results in the database")
	flag.UintVar(&unboundDebug, "unboundDebug", unboundDebug, "Debug level for libunbound")
	flag.BoolVar(&singleWorker, "singleWorker", false, "Limit the number of worker per group to one")
	flag.StringVar(&dbname, "dbName", dbname, "Database name")
	flag.Parse()
	args := flag.Args()

	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] ACTION\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nActions:\n")
		fmt.Fprintln(os.Stderr, "  import-domains: Read domains from stdin for MX lookups")
		fmt.Fprintln(os.Stderr, "  resolve-mx: Read mx records from the domains table and resolve them to A/AAAA records")
		os.Exit(1)
	}

	if singleWorker {
		dnsWorkers = 1
		zgrabWorkers = 1
		domainWorkers = 1
		resultWorkers = 1
		mxWorkers = 1
	}

	dnsProcessor = NewDnsProcessor(dnsWorkers)
	zgrabProcessor = NewZgrabProcessor(zgrabWorkers)
	domainProcessor = NewDomainProcessor(domainWorkers)
	resultProcessor = NewResultProcessor(resultWorkers)
	mxProcessor = NewMxProcessor(mxWorkers)

	if nsupdateKey != "" {
		nsUpdater = NewNsUpdater()
	}

	// Configure DNS
	dnsProcessor.Configure(dnsResolver, dnsTimeout)
	dnsProcessor.unboundCtx.DebugLevel(int(unboundDebug))
	dnsProcessor.unboundCtx.SetOption("num-threads", string(50))

	connect("dbname=" + dbname + " host=/var/run/postgresql")

	gomaxprocs := runtime.NumCPU()
	runtime.GOMAXPROCS(gomaxprocs)
	log.Println("Using", gomaxprocs, "operating system threads")

	// Start control socket handler
	go controlSocket()

	// Process Command
	err := processCommand(args[0], bufio.NewScanner(os.Stdin), bufio.NewWriter(os.Stdout))
	if err != nil {
		os.Stdout.WriteString(err.Error())
	}

	stopProcessors()

	if err != nil {
		os.Exit(1)
	}
}

func stopProcessors() {
	mxProcessor.Close()
	domainProcessor.Close()
	dnsProcessor.Close()
	zgrabProcessor.Close()
	resultProcessor.Close()

	if nsUpdater != nil {
		nsUpdater.Close()
	}
}
