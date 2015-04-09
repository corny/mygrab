package main

import (
	"bytes"
	"database/sql/driver"
	"encoding/hex"
	"strings"
)

type StringArray []string
type ByteaArray [][]byte

func (a StringArray) Value() (driver.Value, error) {
	// FIXME add escaping
	return "{" + strings.Join(a, ",") + "}", nil
}

func (a ByteaArray) Value() (driver.Value, error) {

	buffer := new(bytes.Buffer)
	buffer.WriteString("{")
	for i, elem := range a {
		if i > 0 {
			buffer.WriteString(",")
		}
		buffer.WriteString("\\\\x")
		buffer.WriteString(hex.EncodeToString(elem))
	}
	buffer.WriteString("}")

	return buffer.Bytes(), nil
}
