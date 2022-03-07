package errors

import (
	"strings"
)

// ErrorContext is used to store and manipulate error context strings used
// to output user-friendly, rich information.
type ErrorContext struct {
	contexts []string
}

// NewErrorContext creates an empty ErrorContext.
func NewErrorContext() *ErrorContext { return &ErrorContext{} }

// Add formats and upserts the passed prefix and value onto the error contexts.
func (ctx *ErrorContext) Add(prefix, val string) {
	idx := -1
	for i, c := range ctx.contexts {
		if strings.HasPrefix(c, prefix) {
			idx = i
			break
		}
	}
	if idx != -1 {
		ctx.contexts[idx] = prefix + val
		return
	}

	ctx.contexts = append(ctx.contexts, prefix+val)
}

// Append takes an existing ErrorContext and appends any context into the
// current.
func (ctx *ErrorContext) Append(context *ErrorContext) {
	ctx.contexts = append(ctx.contexts, context.GetAll()...)
}

// Copy to currently stored contexts into a new ErrorContext.
func (ctx *ErrorContext) Copy() *ErrorContext { return &ErrorContext{contexts: ctx.contexts} }

// GetAll returns all the stored context strings.
func (ctx *ErrorContext) GetAll() []string { return ctx.contexts }

// String returns the stored contexts as a minimally formatted string.
func (ctx *ErrorContext) String() string { return strings.Join(ctx.contexts, "\n") }
