package main

import (
	"bytes"
)

type stringSet map[string]struct{}

// creates a comma seperated sorted list
func (set stringSet) String() string {
	buffer := new(bytes.Buffer)
	first := true

	for key, _ := range set {
		if first {
			first = false
		} else {
			buffer.WriteByte(',')
		}
		buffer.WriteString(key)
	}

	return buffer.String()
}

func (set stringSet) Len() int {
	return len(set)
}

func (set stringSet) Add(str string) {
	set[str] = struct{}{}
}
