package helper

// BoolToPtr returns a pointer to the input boolean value.
func BoolToPtr(i bool) *bool { return &i }

// StringToPtr returns the pointer to the input string.
func StringToPtr(str string) *string { return &str }
