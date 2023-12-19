package helper

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// FirstRuneToUpper converts first rune to upper case if necessary.
func FirstRuneToUpper(str string) string {
	if str == "" {
		return str
	}

	r, size := utf8.DecodeRuneInString(str)

	if !unicode.IsLower(r) {
		return str
	}

	buf := strings.Builder{}
	buf.WriteRune(unicode.ToUpper(r))
	buf.WriteString(str[size:])
	return buf.String()
}
