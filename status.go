package main

import (
	"encoding/json"
	"log"
)

// Returns the worker and cache status as JSON
func status() []byte {
	poolStatus := func(pool *WorkerPool) map[string]interface{} {
		m := make(map[string]interface{})
		m["pending"] = len(pool.channel)
		m["processed"] = pool.processed
		m["workers_current"] = pool.currentWorkers
		m["workers_max"] = pool.maxWorkers
		return m
	}

	m := make(map[string]interface{})

	m["dns"] = poolStatus(dnsProcessor.workers)
	m["mx"] = poolStatus(mxProcessor.workers)
	m["zgrab"] = poolStatus(zgrabProcessor.workers)
	m["domain"] = poolStatus(domainProcessor.workers)
	m["result"] = poolStatus(resultProcessor.workers)

	hostCache := make(map[string]interface{})
	hostCache["hits"] = zgrabProcessor.cacheHits
	hostCache["misses"] = zgrabProcessor.cacheMisses
	hostCache["expiries"] = zgrabProcessor.cacheExpiries
	hostCache["refreshes"] = zgrabProcessor.cacheRefreshes
	m["hostCache"] = &hostCache

	result, err := json.Marshal(m)
	if err != nil {
		log.Println(err)
	}

	return result
}
