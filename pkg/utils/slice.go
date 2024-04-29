package utils

// PointSlice converts a slice of values to a slice of pointers to those values.
func PointSlice[T any](slice []T) []*T {
	result := make([]*T, len(slice))
	for i := range slice {
		result[i] = &slice[i]
	}
	return result
}
