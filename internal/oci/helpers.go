// Package oci -- helpers.go: shared deref helpers for pointer types used
// across OCI SDK wrapper files.
package oci

// derefStr returns the string value pointed to by s, or "" if nil.
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// derefInt64 returns the int64 value pointed to by v, or 0 if nil.
func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

// derefFloat32 returns the float32 value pointed to by v, or 0 if nil.
func derefFloat32(v *float32) float32 {
	if v == nil {
		return 0
	}
	return *v
}

// derefFloat64 returns the float64 value pointed to by v, or 0 if nil.
func derefFloat64(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

// derefInt returns the int value pointed to by v, or 0 if nil.
func derefInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
