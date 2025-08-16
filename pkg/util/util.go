// Package util provides common utility functions used across the pinact codebase.
// This package contains reusable helper functions that don't belong to any specific
// domain package. Currently minimal, it provides basic type conversion utilities
// such as pointer creation helpers, following common Go patterns for working with
// APIs that require pointer types.
package util

func StrP(s string) *string {
	return &s
}
