package main

import (
	"github.com/hashicorp/golang-lru"
	"net"
	"testing"
)

func TestZgrabCache(t *testing.T) {
	resultProcessor = NewResultProcessor(0)
	processor := NewZgrabProcessor(1)
	targetA := net.ParseIP("127.0.0.1")
	targetB := net.ParseIP("127.0.0.1")

	processor.NewJob(&targetA)
	processor.Close()

	if processor.cacheMisses != 1 {
		t.Fatal("invalid cache misses: ", processor.cacheMisses)
	}

	processor.NewJob(&targetB)

	if processor.cacheHits != 1 {
		t.Fatal("invalid cache hits: ", processor.cacheHits)
	}
	if processor.cacheMisses != 1 {
		t.Fatal("invalid cache misses: ", processor.cacheMisses)
	}
}

func TestLruCache(t *testing.T) {
	cache, _ := lru.New(10)
	targetA := net.ParseIP("127.0.0.1")
	targetB := net.ParseIP("127.0.0.1")

	cache.Add(targetA.String(), "foo")
	val, _ := cache.Get(targetB.String())

	str, _ := val.(string)

	if str != "foo" {
		t.Fatal("unexpected value:", str)
	}

}
