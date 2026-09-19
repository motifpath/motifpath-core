//go:build integration

package bdd

import "strconv"

func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

func strPtr(s string) *string { return &s }
