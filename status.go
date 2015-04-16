package main

import (
	"encoding/json"
	"log"
)

func status() []byte {
	m := make(map[string]interface{})

	m["dns"] = len(dnsProcessor.workers.channel)
	m["mx"] = len(mxProcessor.workers.channel)
	m["zgrab"] = len(zgrabProcessor.workers.channel)
	m["domain"] = len(domainProcessor.workers.channel)
	m["result"] = len(resultProcessor.workers.channel)

	zgrabStatus := make(map[string]interface{})
	zgrabStatus["size"] = zgrabProcessor.cache.Len()
	zgrabStatus["hits"] = zgrabProcessor.cacheHits
	zgrabStatus["misses"] = zgrabProcessor.cacheMisses
	zgrabStatus["expiries"] = zgrabProcessor.cacheExpiries
	zgrabStatus["concurrentHits"] = zgrabProcessor.concurrentHits
	m["zgrabCache"] = &zgrabStatus

	result, err := json.Marshal(m)
	if err != nil {
		log.Println(err)
	}

	return result
}
