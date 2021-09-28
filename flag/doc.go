// Package flag is a thin layer over the spf13/pflag package. It's copied in
// large part from waypoint's internal flag package, which wraps the stdlib
// flag package and offers some added features such as aliasing, autocompletion
// handling, improved defaults, etc.

// Wrapping the pflag package allows Nom to default to posix-style flags, while
// also offering the stdlib flag as a fallback for compatibility with other
// hashicorp tooling.

// This package follows pflag convention and for every flag type, also has a
// <flagtype>P flag type, which is just the same flag type but with a shorthand
// available. e.g. The StringVar flag is the posix flag without a shorthand, and
// StringVarP is the posix flag with a shorthand.

package flag
