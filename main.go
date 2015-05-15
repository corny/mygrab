package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/zmap/zgrab/zlib"
	"github.com/zmap/zgrab/ztools/zlog"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

var (
	socketPath      string
	zlibConfig           = &zlib.Config{}
	dnsResolver          = "8.8.8.8:53"
	dnsWorkers      uint = 500
	dnsTimeout      uint = 10 // seconds
	zgrabWorkers    uint = 500
	zgrabTimeout    uint = 15
	mxWorkers       uint = 250 // should at least as many as dnsWorkers
	domainWorkers   uint = 250 // should at least as many as dnsWorkers
	resultWorkers   uint = 2
	unboundDebug    uint = 0

	// database settings
	dbName string
	dbHost = "/var/run/postgresql"

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
	flag.UintVar(&zgrabTimeout, "zgrabTimeout", zgrabTimeout, "zgrab timeout in seconds")
	flag.UintVar(&domainWorkers, "domainWorkers", domainWorkers, "Number of dns workers")
	flag.UintVar(&resultWorkers, "resultWorkers", resultWorkers, "Number of result workers that store results in the database")
	flag.UintVar(&unboundDebug, "unboundDebug", unboundDebug, "Debug level for libunbound")
	flag.BoolVar(&singleWorker, "singleWorker", false, "Limit the number of worker per group to one")
	flag.StringVar(&dbName, "dbName", dbName, "Database name. If omitted, not data will be saved.")
	flag.StringVar(&dbHost, "dbHost", dbHost, "Database host or path to unix socket")
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

	zlibConfig.Timeout = time.Duration(zgrabTimeout) * time.Second

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
	mxProcessor = NewMxProcessor(mxWorkers)
	// Configure database
	if dbName != "" {
		connect("dbname=" + dbName + " host=" + dbHost)
		resultProcessor = NewResultProcessor(resultWorkers)
	}

	if nsupdateKey != "" {
		nsUpdater = NewNsUpdater()
	}

	// Configure DNS
	dnsProcessor.Configure(dnsResolver, dnsTimeout)
	dnsProcessor.unboundCtx.DebugLevel(int(unboundDebug))
	dnsProcessor.unboundCtx.SetOption("num-threads", string(50))

	gomaxprocs := runtime.NumCPU()
	runtime.GOMAXPROCS(gomaxprocs)
	log.Println("Using", gomaxprocs, "operating system threads")

	// Start control socket handler
	go controlSocket()

	command := args[0]
	var err error

	if command == "daemon" {
		// Wait for SIGINT or SIGTERM
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		<-sigs
	} else {
		// Process Command
		err = processCommand(args[0], bufio.NewScanner(os.Stdin), bufio.NewWriter(os.Stdout))
		if err != nil {
			os.Stdout.WriteString(err.Error())
		}
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
