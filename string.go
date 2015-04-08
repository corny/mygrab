package main

// Returns the unique elements
func UniqueStrings(list []string) []string {
	m := make(map[string]bool)
	result := make([]string, 0)

	// insert list as keys
	for _, item := range list {
		m[item] = true
	}

	// collect keys
	for _, item := range list {
		result = append(result, item)
	}
	return result
}
