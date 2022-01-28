package terminal

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"
	"github.com/mitchellh/go-glint"
)

// ErrNonInteractive is returned when Input is called on a non-Interactive UI.
var ErrNonInteractive = errors.New("noninteractive UI doesn't support this operation")

// Passed to UI.NamedValues to provide a nicely formatted key: value output
type NamedValue struct {
	Name  string
	Value interface{}
}

// UI is the primary interface for interacting with a user via the CLI.
//
// Some of the methods on this interface return values that have a lifetime
// such as Status and StepGroup. While these are still active (haven't called
// the close or equivalent method on these values), no other method on the
// UI should be called.
type UI interface {
	// Input asks the user for input. This will immediately return an error
	// if the UI doesn't support interaction. You can test for interaction
	// ahead of time with Interactive().
	Input(*Input) (string, error)

	// Interactive returns true if this prompt supports user interaction.
	// If this is false, Input will always error.
	Interactive() bool

	// Output outputs a message directly to the terminal. The remaining
	// arguments should be interpolations for the format string. After the
	// interpolations you may add Options.
	Output(string, ...interface{})

	// AppendToRow appends a message to a row of output. Used for applying multiple
	// styles to a single row of text.
	AppendToRow(string, ...interface{})

	// NamedValues outputs data as a table of data. Each entry is a row which will be output
	// with the columns lined up nicely.
	NamedValues([]NamedValue, ...Option)

	// OutputWriters returns stdout and stderr writers. These are usually
	// but not always TTYs. This is useful for subprocesses, network requests,
	// etc. Note that writing to these is not thread-safe by default so
	// you must take care that there is only ever one writer.
	OutputWriters() (stdout, stderr io.Writer, err error)

	// Status returns a live-updating status that can be used for single-line
	// status updates that typically have a spinner or some similar style.
	// While a Status is live (Close isn't called), other methods on UI should
	// NOT be called.
	Status() Status

	// Table outputs the information formatted into a Table structure.
	Table(*Table, ...Option)

	// StepGroup returns a value that can be used to output individual (possibly
	// parallel) steps that have their own message, status indicator, spinner, and
	// body. No other output mechanism (Output, Input, Status, etc.) may be
	// called until the StepGroup is complete.
	StepGroup() StepGroup

	// Debug formats output with the DebugStyle
	Debug(string)

	// Error formats Output with the ErrorStyle
	Error(string)

	// ErrorWithContext formats an error output including additional context so
	// users can easily identify issues.
	ErrorWithContext(err error, sub string, ctx ...string)

	// Header formats Output with the HeaderStyle
	Header(string)

	// Info formats Output with the InfoStyle
	Info(string)

	// Success formats Output with the SuccessStyle
	Success(string)

	// Trace formats Output with the TraceStyle
	Trace(string)

	// Warning formats Output with the WarningStyle
	Warning(string)

	// WarningBold formats Output with the WarningBoldStyle
	WarningBold(string)
}

// StepGroup is a group of steps (that may be concurrent).
type StepGroup interface {
	// Start a step in the output with the arguments making up the initial message
	Add(string, ...interface{}) Step

	// Wait for all steps to finish. This allows a StepGroup to be used like
	// a sync.WaitGroup with each step being run in a separate goroutine.
	// This must be called to properly clean up the step group.
	Wait()
}

// A Step is the unit of work within a StepGroup. This can be driven by concurrent
// goroutines safely.
type Step interface {
	// The Writer has data written to it as though it was a terminal. This will appear
	// as body text under the Step's message and status.
	TermOutput() io.Writer

	// Change the Steps displayed message
	Update(string, ...interface{})

	// Update the status of the message. Supported values are in status.go.
	Status(status string)

	// Called when the step has finished. This must be done otherwise the StepGroup
	// will wait forever for it's Steps to finish.
	Done()

	// Sets the status to Error and finishes the Step if it's not already done.
	// This is usually done in a defer so that any return before the Done() shows
	// the Step didn't completely properly.
	Abort()
}

// Interpret decomposes the msg and arguments into the message, style, and writer
func Interpret(msg string, raw ...interface{}) (string, string, io.Writer) {
	// Build our args and options
	var args []interface{}
	var opts []Option
	for _, r := range raw {
		if opt, ok := r.(Option); ok {
			opts = append(opts, opt)
		} else {
			args = append(args, r)
		}
	}

	// Build our message
	msg = fmt.Sprintf(msg, args...)

	// Build our config and set our options
	cfg := &config{Writer: color.Output}
	for _, opt := range opts {
		opt(cfg)
	}

	return msg, cfg.Style, cfg.Writer
}

const (
	HeaderStyle      = "header"
	DebugStyle       = "debug"
	ErrorStyle       = "error"
	ErrorBoldStyle   = "error-bold"
	TraceStyle       = "trace"
	WarningStyle     = "warning"
	WarningBoldStyle = "warning-bold"
	InfoStyle        = "info"
	SuccessStyle     = "success"
	SuccessBoldStyle = "success-bold"
	BoldStyle        = "bold"

	BlueStyle        = "blue"
	CyanStyle        = "cyan"
	GreenStyle       = "green"
	RedStyle         = "red"
	YellowStyle      = "yellow"
	LightYellowStyle = "light-yellow"

	DefaultStyle = "default"
)

type config struct {
	// Writer is where the message will be written to.
	Writer io.Writer

	// The style the output should take on
	Style string
}

// Option controls output styling.
type Option func(*config)

// WithHeaderStyle styles the output like a header denoting a new section
// of execution. This should only be used with single-line output. Multi-line
// output will not look correct.
func WithHeaderStyle() Option {
	return func(c *config) {
		c.Style = HeaderStyle
	}
}

// WithInfoStyle styles the output like it's formatted information.
func WithInfoStyle() Option {
	return func(c *config) {
		c.Style = InfoStyle
	}
}

// WithDebugStyle styles the output as a debug message.
func WithDebugStyle() Option {
	return func(c *config) {
		c.Style = DebugStyle
	}
}

// WithErrorStyle styles the output as an error message.
func WithErrorStyle() Option {
	return func(c *config) {
		c.Style = ErrorStyle
	}
}

// WithTraceStyle styles the output as an error message.
func WithTraceStyle() Option {
	return func(c *config) {
		c.Style = TraceStyle
	}
}

// WithWarningStyle styles the output as an error message.
func WithWarningStyle() Option {
	return func(c *config) {
		c.Style = WarningStyle
	}
}

// WithSuccessStyle styles the output as a success message.
func WithSuccessStyle() Option {
	return func(c *config) {
		c.Style = SuccessStyle
	}
}

func WithStyle(style string) Option {
	return func(c *config) {
		c.Style = style
	}
}

// WithWriter specifies the writer for the output.
func WithWriter(w io.Writer) Option {
	return func(c *config) { c.Writer = w }
}

func ErrorWithContext(err error, sub string, ctx ...string) {

	// Create a new glint document.
	d := glint.New()

	// Title the error output in red with the subject.
	d.Append(glint.Layout(
		glint.Style(
			glint.Text(fmt.Sprintf("! %s\n", strings.Title(sub))),
			glint.Color("red"),
		),
	).Row())

	// Add the error string as well as the error type to the output.
	d.Append(glint.Layout(
		glint.Style(glint.Text("\tError:   "), glint.Bold()),
		glint.Text(err.Error()),
	).Row())

	d.Append(glint.Layout(
		glint.Style(glint.Text("\tType:    "), glint.Bold()),
		glint.Text(fmt.Sprintf("%T", err)),
	).Row())

	// We only want this section once per error output, so we cannot perform
	// this within the ctx loop.
	if len(ctx) > 0 {
		d.Append(glint.Layout(
			glint.Style(glint.Text("\tContext: "), glint.Bold()),
		).Row())
	}

	// Iterate the addition context items and append these to the output.
	for _, additionCTX := range ctx {
		d.Append(glint.Layout(
			glint.Style(glint.Text(fmt.Sprintf("\t         - %s", additionCTX))),
		).Row())
	}

	d.RenderFrame()
}

var (
	colorHeader      = color.New(color.Bold)
	colorDebug       = color.New(color.FgHiBlue)
	colorInfo        = color.New()
	colorError       = color.New(color.FgRed)
	colorErrorBold   = color.New(color.FgRed, color.Bold)
	colorSuccess     = color.New(color.FgGreen)
	colorSuccessBold = color.New(color.FgGreen, color.Bold)
	colorTrace       = color.New(color.FgCyan)
	colorWarning     = color.New(color.FgYellow)
	colorWarningBold = color.New(color.FgYellow, color.Bold)
)
