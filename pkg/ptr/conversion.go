package ptr

// BoolToPtr converts a bool to a bool pointer.
func BoolToPtr(b bool) *bool {
	return &b
}

// Int64ToPtr converts an int64 to an int64 pointer.
func Int64ToPtr(i int64) *int64 {
	return &i
}

// Int32ToPtr converts an int32 to an int32 pointer.
func Int32ToPtr(i int32) *int32 {
	return &i
}
