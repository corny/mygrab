package main

import (
	"testing"
)

func TestByteaArray(t *testing.T) {
	// string -> byte array -> ByteaArray
	word := []byte("foo")
	arr := ByteaArray([][]byte{word, []byte{15}})

	// ByteaArray.Value() -> byte array -> string
	val, _ := arr.Value()
	bytes, _ := val.([]byte)
	str := string(bytes)

	if str != "{\\\\x666f6f,\\\\x0f}" {
		t.Fatal("unexpected value:", str)
	}
}
