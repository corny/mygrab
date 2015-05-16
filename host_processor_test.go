package main

import (
	"github.com/hashicorp/golang-lru"
	"net"
	"testing"
)

func TestHostConcurrency(t *testing.T) {
	processor := NewHostProcessor(0, nil)
	targetA := net.ParseIP("127.0.0.1")
	targetB := net.ParseIP("127.0.0.1")

	processor.NewJob(targetA)
	processor.NewJob(targetB)

	if processor.cache.cacheHits != 1 {
		t.Fatal("invalid concurrent Hits: ", processor.cache.cacheHits)
	}

	length := len(processor.cache.workers.channel)
	if length != 1 {
		t.Fatal("invalid channel length: ", length)
	}
}

func TestHostCache(t *testing.T) {
	processor := NewHostProcessor(1, nil)
	targetA := net.ParseIP("127.0.0.1")
	targetB := net.ParseIP("127.0.0.1")

	processor.NewJob(targetA)
	processor.Close()

	if processor.cache.cacheMisses != 1 {
		t.Fatal("invalid cache misses: ", processor.cache.cacheMisses)
	}

	processor.NewJob(targetB)

	if processor.cache.cacheHits != 1 {
		t.Fatal("invalid cache Hits: ", processor.cache.cacheHits)
	}
	if processor.cache.cacheMisses != 1 {
		t.Fatal("invalid cache misses: ", processor.cache.cacheMisses)
	}
}

func TestLruCache(t *testing.T) {
	cache, _ := lru.New(10)
	targetA := string(net.ParseIP("127.0.0.1"))
	targetB := string(net.ParseIP("127.0.0.1"))

	cache.Add(targetA, "foo")
	val, _ := cache.Get(targetB)

	str, _ := val.(string)

	if str != "foo" {
		t.Fatal("unexpected value:", str)
	}

}
