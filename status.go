package main

import (
	"encoding/json"
	"log"
)

func status() []byte {
	m := make(map[string]interface{})

	m["dns"] = len(dnsProcessor.workers.channel)
	m["zgrab"] = len(zgrabProcessor.workers.channel)
	m["domain"] = len(domainProcessor.workers.channel)
	m["result"] = len(resultProcessor.workers.channel)

	result, err := json.Marshal(m)
	if err != nil {
		log.Println(err)
	}

	return result
}
