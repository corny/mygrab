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
	socketPath         string
	zlibConfig              = &zlib.Config{}
	dnsZone                 = "."
	dnsResolver             = "8.8.8.8:53"
	dnsResolverTimeout uint = 10 // seconds
	dnsTTL             uint = 600
	dnsWorkers         uint = 500
	dnsServerEnable    bool
	dnsUseUnbound      bool
	mxWorkers          uint = 250 // should at least as many as dnsWorkers
	domainWorkers      uint = 250 // should at least as many as dnsWorkers
	resultWorkers      uint = 2
	unboundDebug       uint = 0
	unboundTaFile           = "/etc/unbound/root.key"

	// host settings
	hostCacheEnable   bool
	hostCacheExpires  uint = 3600
	hostCacheRefresh  uint = 0
	hostCacheInterval uint = 60
	hostWorkers       uint = 500
	hostTimeout       uint = 15

	// mx cache
	mxCacheEnable   bool
	mxCacheExpires  uint = 3600
	mxCacheRefresh  uint = 0
	mxCacheInterval uint = 60

	// database settings
	dbName string
	dbUser string
	dbHost = "/var/run/postgresql"

	dnsProcessor    *DnsProcessor    // dns lookups
	hostProcessor   *HostProcessor   // host checks
	domainProcessor *DomainProcessor // uses the dnsProcessor for MX lookups and saves the domain
	mxProcessor     *MxProcessor     // uses the dnsProcessor for A/AAAA lookups, the hostProcessor for hostChecks and creates TXT records
	resultProcessor *ResultProcessor // stores results in a postgres database
	nsUpdater       *NsUpdater       // passes txt records to nsupdate
	dnsServer       *DnsServer       // creates a DNS server
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
	var err error
	var command string
	var singleWorker bool

	flags := flag.NewFlagSet("default", flag.ContinueOnError)

	flags.StringVar(&zlibConfig.EHLODomain, "ehlo", zlibConfig.EHLODomain, "Send an EHLO with the specified domain (implies --smtp)")
	flags.StringVar(&dnsResolver, "dnsResolver", dnsResolver, "DNS resolver address")
	flags.UintVar(&dnsResolverTimeout, "dnsResolverTimeout", dnsResolverTimeout, "DNS timeout in seconds")
	flags.StringVar(&dnsZone, "dnsZone", dnsZone, "The zone for nsupdate and the internal DNS server. 'example.com' will serve a TXT record for some-domain.com at 'some-domain.com.example.com'.")
	flags.UintVar(&dnsTTL, "dnsTTL", dnsTTL, "TTL for DNS dns records")
	flags.BoolVar(&dnsServerEnable, "dnsServerEnable", dnsServerEnable, "Enable the internal dns server")
	flags.BoolVar(&dnsUseUnbound, "dnsUseUnbound", dnsUseUnbound, "Use libunbound as recursive resolver to get additional DNSSEC information")

	// nsupdate
	flags.StringVar(&nsupdateKey, "nsupdateKey", "", "path to nsupdate key. If ommited, no updates will happen.")
	flags.StringVar(&nsupdateServer, "nsupdateServer", nsupdateServer, "dns server for nsupdate")

	// host cache
	flags.BoolVar(&hostCacheEnable, "hostCacheEnable", hostCacheEnable, "Always true if dnsServer is enabled or command is 'import-mx'")
	flags.UintVar(&hostCacheExpires, "hostCacheExpires", hostCacheExpires, "A host result will be removed after this number of seconds not accessed. A value of 0 disables the cache.")
	flags.UintVar(&hostCacheRefresh, "hostCacheRefresh", hostCacheRefresh, "A host result will be refreshed after this number of seconds. A value of 0 means it will never be refreshed.")
	flags.UintVar(&hostCacheInterval, "hostCacheInterval", hostCacheInterval, "The cache worker will sleep for this duration of seconds between runs.")

	// mx cache
	flags.BoolVar(&mxCacheEnable, "mxCacheEnable", mxCacheEnable, "Always true if dnsServer is enabled or command is 'import-mx'")
	flags.UintVar(&mxCacheExpires, "mxCacheExpires", mxCacheExpires, "A/AAAA will be removed after this number of seconds not accessed. A value of 0 disables the cache.")
	flags.UintVar(&mxCacheRefresh, "mxCacheRefresh", mxCacheRefresh, "A mxCache result will be refreshed after this number of seconds. A value of 0 means it will never be refreshed.")
	flags.UintVar(&mxCacheInterval, "mxCacheInterval", mxCacheInterval, "The cache worker will sleep for this duration of seconds between runs.")

	flags.StringVar(&socketPath, "socket", "", "Path for the control unix socket")
	flags.UintVar(&dnsWorkers, "dnsWorkers", dnsWorkers, "Number of dns workers")
	flags.UintVar(&mxWorkers, "mxWorkers", mxWorkers, "Number of mx workers")
	flags.UintVar(&hostWorkers, "hostWorkers", hostWorkers, "Number of zgrab workers")
	flags.UintVar(&hostTimeout, "hostTimeout", hostTimeout, "zgrab timeout in seconds")
	flags.UintVar(&domainWorkers, "domainWorkers", domainWorkers, "Number of dns workers")
	flags.UintVar(&resultWorkers, "resultWorkers", resultWorkers, "Number of result workers that store results in the database")
	flags.UintVar(&unboundDebug, "unboundDebug", unboundDebug, "Debug level for libunbound")
	flags.StringVar(&unboundTaFile, "unboundTaFile", unboundTaFile, "Trusted anchor file for libunbound")
	flags.BoolVar(&singleWorker, "singleWorker", false, "Limit the number of workers per worker pool to one")

	flags.StringVar(&dbName, "dbName", dbName, "Database name. If omitted, not data will be saved.")
	flags.StringVar(&dbHost, "dbHost", dbHost, "Database host or path to unix socket")
	flags.StringVar(&dbUser, "dbUser", dbUser, "Database user")

	err = flags.Parse(os.Args[1:])
	args := flags.Args()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] ACTION\n\nOptions:\n", os.Args[0])
		flags.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nActions:\n")
		fmt.Fprintln(os.Stderr, "  import-domains: Read domains from stdin for MX lookups")
		fmt.Fprintln(os.Stderr, "  import-addresses: Read ip addresses from stdin and run host checks. Cache will be disabled.")
		fmt.Fprintln(os.Stderr, "  resolve-mx: Read mx records from the domains table and resolve them to A/AAAA records")
		// TODO document all commands
		os.Exit(1)
	}
	if len(args) == 1 {
		command = args[0]
	}

	if hostTimeout == 0 {
		log.Fatalln("hostTimeout must be > 0")

	}
	zlibConfig.Timeout = time.Duration(hostTimeout) * time.Second

	if singleWorker {
		dnsWorkers = 1
		hostWorkers = 1
		domainWorkers = 1
		resultWorkers = 1
		mxWorkers = 1
	}

	switch command {
	case "import-addresses":
		// disable host cache: we assume that the input has not duplicates
		hostCacheExpires = 0
	case "import-mx", "resolve-mx":
		// enable host cache: resolving mx records leads to many duplicate hosts
		hostCacheEnable = true
		hostCacheRefresh = 0
		// disable mx cache: there can not be duplicate mx hostnames
		mxCacheEnable = false
	}

	// Configure database
	if dbName != "" {
		connect("dbname=" + dbName + " host=" + dbHost + " user=" + dbUser)
		resultProcessor = NewResultProcessor(resultWorkers)
	}

	// Configure NsUpdater
	if nsupdateKey != "" {
		nsUpdater = NewNsUpdater()
	}

	// Configure caching
	if dnsServerEnable {
		hostCacheEnable = true
		mxCacheEnable = true
	}
	var mxCache *CacheConfig
	var hostCache *CacheConfig

	if hostCacheEnable {
		if hostCacheInterval == 0 {
			log.Fatalln("hostCacheInterval must be > 0")
		}
		hostCache = NewCacheConfig(hostCacheExpires, hostCacheRefresh, hostCacheInterval)
	}

	if mxCacheEnable {
		if mxCacheInterval == 0 {
			log.Fatalln("mxCacheInterval must be > 0")
		}
		mxCache = NewCacheConfig(mxCacheExpires, mxCacheRefresh, mxCacheInterval)
	}

	dnsProcessor = NewDnsProcessor(dnsWorkers)
	domainProcessor = NewDomainProcessor(domainWorkers)
	hostProcessor = NewHostProcessor(hostWorkers, hostCache)
	mxProcessor = NewMxProcessor(mxWorkers, mxCache)

	// Configure DNS
	if dnsUseUnbound {
		if _, err := os.Stat(unboundTaFile); os.IsNotExist(err) {
			log.Fatalln("unboundTaFile", unboundTaFile, "does not exist")
		}
		dnsProcessor.ConfigureUnbound(unboundDebug, unboundTaFile)
	} else {
		dnsProcessor.Configure(dnsResolver, dnsResolverTimeout)
	}

	// Configure number of system threads
	gomaxprocs := runtime.NumCPU()
	runtime.GOMAXPROCS(gomaxprocs)
	log.Println("Using", gomaxprocs, "operating system threads")

	// Start control socket handler
	if socketPath != "" {
		go controlSocket()
	}

	if dnsServerEnable {
		// Start the DNS server
		dnsServer = NewDnsServer(dnsZone)
	}

	if command == "" {
		// Wait for SIGINT or SIGTERM
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigs
		log.Println("received", sig)
	} else {
		// Process Command
		err = processCommand(args[0], bufio.NewScanner(os.Stdin), bufio.NewWriter(os.Stdout))
	}

	stopProcessors()

	if err != nil {
		os.Stdout.WriteString(err.Error())
		os.Exit(1)
	}
}

func stopProcessors() {
	mxProcessor.Close()
	domainProcessor.Close()
	dnsProcessor.Close()
	hostProcessor.Close()

	if resultProcessor != nil {
		resultProcessor.Close()
	}

	if dnsServer != nil {
		dnsServer.Close()
	}

	if nsUpdater != nil {
		nsUpdater.Close()
	}
}
