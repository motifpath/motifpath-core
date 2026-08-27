//go:build integration

package bdd

import "strconv"

func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}
