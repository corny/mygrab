package main

import (
	"encoding/json"
	"log"
)

func status() []byte {
	poolStatus := func(pool *WorkerPool) map[string]interface{} {
		m := make(map[string]interface{})
		m["pending"] = len(pool.channel)
		m["processed"] = pool.processed
		return m
	}

	m := make(map[string]interface{})

	m["dns"] = poolStatus(dnsProcessor.workers)
	m["mx"] = poolStatus(mxProcessor.workers)
	m["zgrab"] = poolStatus(zgrabProcessor.workers)
	m["domain"] = poolStatus(domainProcessor.workers)
	m["result"] = poolStatus(resultProcessor.workers)

	zgrabStatus := make(map[string]interface{})
	zgrabStatus["hits"] = zgrabProcessor.cacheHits
	zgrabStatus["misses"] = zgrabProcessor.cacheMisses
	zgrabStatus["expiries"] = zgrabProcessor.cacheExpiries
	zgrabStatus["refreshes"] = zgrabProcessor.cacheRefreshes
	zgrabStatus["concurrentHits"] = zgrabProcessor.concurrentHits
	m["zgrabCache"] = &zgrabStatus

	result, err := json.Marshal(m)
	if err != nil {
		log.Println(err)
	}

	return result
}
