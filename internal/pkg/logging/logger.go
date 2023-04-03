// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package logging

import (
	"fmt"
)

// Logger is the primary interface for logging with a consistent interface without
// creating a hard dependency between the UI layer and lower layers of the stack.
// It is inspired a subset of the functions defined by terminal.UI which are generic
// enough for lower level packages to consume. It expected that implementations
// of this interface will respect the NOMAD_PACK_LOG_LEVEL environment variable.
type Logger interface {
	// Debug logs at the DEBUG log level
	Debug(message string)

	// Error logs at the ERROR log level
	Error(message string)

	// ErrorWithContext logs at the ERROR log level including additional context so
	// users can easily identify issues.
	ErrorWithContext(err error, sub string, ctx ...string)

	// Info logs at the INFO log level
	Info(message string)

	// Trace logs at the TRACE log level
	Trace(message string)

	// Warning logs at the WARN log level
	Warning(message string)
}

type FmtLogger struct{}

// Debug logs at the DEBUG log level
func (l *FmtLogger) Debug(message string) {
	fmt.Println(message)
}

// Error logs at the ERROR log level
func (l *FmtLogger) Error(message string) {
	fmt.Println(message)
}

// ErrorWithContext logs at the ERROR log level including additional context so
// users can easily identify issues.
func (l *FmtLogger) ErrorWithContext(err error, sub string, ctx ...string) {
	fmt.Printf("err: %s\n", err)
	fmt.Println(sub)

	for _, entry := range ctx {
		fmt.Println(entry)
	}
}

// Info logs at the INFO log level
func (l *FmtLogger) Info(message string) {
	fmt.Println(message)
}

// Trace logs at the TRACE log level
func (l *FmtLogger) Trace(message string) {
	fmt.Println(message)
}

// Warning logs at the WARN log level
func (l *FmtLogger) Warning(message string) {
	fmt.Println(message)
}

func Default() *FmtLogger {
	return &FmtLogger{}
}

type TestLogger struct {
	log func(args ...interface{})
}

// Debug logs at the DEBUG log level
func (l *TestLogger) Debug(message string) {
	l.log(message)
}

// Error logs at the ERROR log level
func (l *TestLogger) Error(message string) {
	l.log(message)
}

// ErrorWithContext logs at the ERROR log level including additional context so
// users can easily identify issues.
func (l *TestLogger) ErrorWithContext(err error, sub string, ctx ...string) {
	l.log(fmt.Sprintf("err: %s", err))
	l.log(sub)

	for _, entry := range ctx {
		l.log(entry)
	}
}

// Info logs at the INFO log level
func (l *TestLogger) Info(message string) {
	l.log(message)
}

// Trace logs at the TRACE log level
func (l *TestLogger) Trace(message string) {
	l.log(message)
}

// Warning logs at the WARN log level
func (l *TestLogger) Warning(message string) {
	l.log(message)
}

// NewTestLogger returns a test logger suitable for use with the go testing.T log function.
func NewTestLogger(log func(args ...interface{})) *TestLogger {
	return &TestLogger{
		log: log,
	}
}
