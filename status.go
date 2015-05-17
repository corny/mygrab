package main

import (
	"encoding/json"
)

// Returns the worker and cache status as JSON
func status() ([]byte, error) {
	poolStatus := func(pool *WorkerPool) map[string]interface{} {
		m := make(map[string]interface{})
		m["pending"] = len(pool.channel)
		m["processed"] = pool.processed
		m["workers_current"] = pool.currentWorkers
		m["workers_max"] = pool.maxWorkers
		return m
	}

	cacheStatus := func(pool *CachedWorkerPool) map[string]interface{} {
		c := make(map[string]interface{})
		c["entries"] = len(pool.cache)
		c["stats"] = pool
		c["config"] = pool.cacheConfig
		m := poolStatus(pool.workers)
		m["cache"] = c
		return m
	}

	m := make(map[string]interface{})
	m["dns"] = poolStatus(dnsProcessor.workers)
	m["domain"] = poolStatus(domainProcessor.workers)
	m["host"] = cacheStatus(hostProcessor.cache)
	m["mx"] = cacheStatus(mxProcessor.cache)
	if resultProcessor != nil {
		m["result"] = poolStatus(resultProcessor.workers)
	}

	return json.Marshal(m)
}

type KeyConverter func(string) string

// Returns the cache content for a CachedWorkerPool as JSON
func cacheStatus(cache *CachedWorkerPool, converter KeyConverter) ([]byte, error) {
	m := make(map[string]interface{})
	for key, entry := range cache.cache {
		if converter != nil {
			key = converter(key)
		}
		m[key] = entry
	}
	return json.Marshal(m)
}
