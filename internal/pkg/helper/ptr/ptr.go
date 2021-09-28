package ptr

// Bool returns a pointer to the input boolean value.
func Bool(i bool) *bool { return &i }
