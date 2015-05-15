package main

import (
	"bytes"
	"database/sql/driver"
	"encoding/hex"
	"sort"
	"strings"
)

type StringArray []string
type ByteaArray [][]byte

func (a StringArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return nil, nil
	} else {
		// FIXME add escaping #security
		return "{\"" + strings.Join(a, "\",\"") + "\"}", nil
	}
}

func (a ByteaArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return nil, nil
	}

	// Sorts elements to avoid random order
	elements := make([]string, len(a))
	for i, elem := range a {
		elements[i] = string(elem)
	}
	sort.Strings(elements)

	// Build the output
	buffer := new(bytes.Buffer)
	buffer.WriteString("{")
	for i, elem := range elements {
		if i > 0 {
			buffer.WriteString(",")
		}
		buffer.WriteString("\\\\x")
		buffer.WriteString(hex.EncodeToString([]byte(elem)))
	}
	buffer.WriteString("}")

	return buffer.Bytes(), nil
}

func StringsToByteArray(strings []string) [][]byte {
	arr := make([][]byte, len(strings))

	for i, str := range strings {
		arr[i] = []byte(str)
	}

	return arr
}
