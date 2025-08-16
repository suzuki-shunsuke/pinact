// Package util provides common utility functions used across the pinact codebase.
// This package contains reusable helper functions that don't belong to any specific
// domain package. Currently minimal, it provides basic type conversion utilities
// such as pointer creation helpers, following common Go patterns for working with
// APIs that require pointer types.
package util

// StrP returns a pointer to the provided string value.
// This is a convenience function for creating string pointers, commonly
// needed when working with APIs that require optional string fields.
//
// Parameters:
//   - s: string value to get a pointer to
//
// Returns a pointer to the string value.
func StrP(s string) *string {
	return &s
}
