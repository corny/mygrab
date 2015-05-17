package main

import (
	"github.com/deckarep/golang-set"
	"testing"
)

func TestSplitByLength(t *testing.T) {
	var arr []string

	arr = SplitByLength("", 3)
	if len(arr) != 0 {
		t.Fatal("unexpected value:", arr)
	}

	arr = SplitByLength("foobar", 3)
	if len(arr) != 2 || arr[0] != "foo" || arr[1] != "bar" {
		t.Fatal("unexpected value:", arr)
	}

	arr = SplitByLength("foobar", 4)
	if len(arr) != 2 || arr[0] != "foob" || arr[1] != "ar" {
		t.Fatal("unexpected value:", arr)
	}

	arr = SplitByLength("foobar", 6)
	if len(arr) != 1 || arr[0] != "foobar" {
		t.Fatal("unexpected value:", arr)
	}

	arr = SplitByLength("foobar", 7)
	if len(arr) != 1 || arr[0] != "foobar" {
		t.Fatal("unexpected value:", arr)
	}
}

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
