package main

import (
	"bytes"
	"encoding/hex"
	"github.com/deckarep/golang-set"
)

// Returns the unique elements
func UniqueStrings(list []string) []string {
	m := make(map[string]bool)
	result := make([]string, 0)

	// insert list as keys
	for _, item := range list {
		m[item] = true
	}

	// collect keys
	for item, _ := range m {
		result = append(result, item)
	}
	return result
}

// creates a comma seperated sorted list
func joinSet(set mapset.Set, hexEncode bool) string {
	buffer := new(bytes.Buffer)
	first := true

	for item := range set.Iter() {
		if first {
			first = false
		} else {
			buffer.WriteByte(',')
		}

		value := item.(string)
		if hexEncode {
			value = hex.EncodeToString([]byte(value))
		}
		buffer.WriteString(value)
	}

	return buffer.String()
}

// creates a comma seperated sorted list
func setToByteArrays(set mapset.Set) [][]byte {
	items := set.ToSlice()
	result := make([][]byte, len(items))

	for i, item := range items {
		value := item.(string)
		result[i] = []byte(value)
	}

	return result
}
