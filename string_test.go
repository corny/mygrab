package main

import (
	"github.com/deckarep/golang-set"
	"testing"
)

func TestJoinStringSet(t *testing.T) {

	slice := []interface{}{"foo", "bar"}
	set := mapset.NewThreadUnsafeSetFromSlice(slice)
	str := joinSet(set, false)

	// seed for maps is random
	if str != "bar,foo" && str != "foo,bar" {
		t.Fatal("unexpected value:", str)
	}
}

func TestJoinByteSet(t *testing.T) {

	slice := []interface{}{"foo", "ba"}
	set := mapset.NewThreadUnsafeSetFromSlice(slice)
	str := joinSet(set, true)

	// seed for maps is random
	if str != "666f6f,6261" && str != "6261,666f6f" {
		t.Fatal("unexpected value:", str)
	}
}
