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
	dnsWorkers      uint = 100
	dnsTimeout      uint = 10 // seconds
	zgrabWorkers    uint = 500
	domainWorkers   uint = 100 // should be more at least as many as dnsWorkers
	resultWorkers   uint = 2
	unboundDebug    uint = 0
	dbname          string
	dnsProcessor    *DnsProcessor
	zgrabProcessor  *ZgrabProcessor
	domainProcessor *DomainProcessor
	resultProcessor *ResultProcessor
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
}

func main() {

	flag.StringVar(&zlibConfig.EHLODomain, "ehlo", "example.com", "Send an EHLO with the specified domain (implies --smtp)")
	flag.StringVar(&dnsResolver, "dnsResolver", dnsResolver, "DNS resolver address")
	flag.UintVar(&dnsTimeout, "dnsTimeout", dnsTimeout, "DNS timeout in seconds")
	flag.StringVar(&socketPath, "socket", "", "Read from a socket instead of stdin")
	flag.UintVar(&dnsWorkers, "dnsWorkers", dnsWorkers, "Number of dns workers")
	flag.UintVar(&zgrabWorkers, "zgrabWorkers", zgrabWorkers, "Number of zgrab workers")
	flag.UintVar(&domainWorkers, "domainWorkers", domainWorkers, "Number of dns workers")
	flag.UintVar(&resultWorkers, "resultWorkers", resultWorkers, "Number of result workers that store results in the database")
	flag.UintVar(&unboundDebug, "unboundDebug", unboundDebug, "Debug level for libunbound")
	flag.StringVar(&dbname, "dbName", dbname, "Database name")
	flag.Parse()
	args := flag.Args()

	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] ACTION\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nActions:\n")
		fmt.Fprintln(os.Stderr, "  import-domains: Read domains from stdin for MX lookups")
		fmt.Fprintln(os.Stderr, "  domains-to-mx: Read mx records from the domains table")
		os.Exit(1)
	}

	dnsProcessor = NewDnsProcessor(dnsWorkers)
	zgrabProcessor = NewZgrabProcessor(zgrabWorkers)
	domainProcessor = NewDomainProcessor(domainWorkers)
	resultProcessor = NewResultProcessor(resultWorkers)

	// Configure DNS
	dnsProcessor.Configure(dnsResolver, dnsTimeout)
	dnsProcessor.unboundCtx.DebugLevel(int(unboundDebug))
	dnsProcessor.unboundCtx.SetOption("num-threads", string(50))

	connect("dbname=" + dbname + " host=/var/run/postgresql")

	gomaxprocs := runtime.NumCPU()
	runtime.GOMAXPROCS(gomaxprocs)
	log.Println("Using", gomaxprocs, "operating system threads")

	go processSocket()

	action := args[0]
	switch action {
	case "import-domains":
		// Read stdin
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			domainProcessor.Add(scanner.Text())
		}
	default:
		fmt.Fprintln(os.Stderr, "Unknown action:", action)
	}

	stopProcessors()
}

func stopProcessors() {
	domainProcessor.Close()
	dnsProcessor.Close()
	zgrabProcessor.Close()
	resultProcessor.Close()
}
