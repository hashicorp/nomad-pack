// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/containerd/console"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/mitchellh/cli"
	"golang.org/x/term"
)

// ErrNonInteractive is returned when Input is called on a non-Interactive UI.
var ErrNonInteractive = errors.New("noninteractive UI doesn't support this operation")

// UsageCommander is an interface for commands that supply a terse help messsage
// that points to the specific command's --help flag.
type UsageCommander interface {
	HelpUsageMessage() string
}

// Passed to UI.NamedValues to provide a nicely formatted key: value output
type NamedValue struct {
	Name  string
	Value any
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
	Output(string, ...any)

	// AppendToRow appends a message to a row of output. Used for applying multiple
	// styles to a single row of text.
	AppendToRow(string, ...any)

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

	// ErrorWithUsageAndContext displays both an error and the usage. This should be
	// called when flag and argument parsing fail.
	ErrorWithUsageAndContext(err error, sub string, c cli.Command, ctx ...string)

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
	Add(string, ...any) Step

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
	Update(string, ...any)

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

// Returns a UI which will write to the current processes
// stdout/stderr.
func ConsoleUI(ctx context.Context) UI {
	// We do both of these checks because some sneaky environments fool
	// one or the other and we really only want the glint-based UI in
	// truly interactive environments.
	glint := isatty.IsTerminal(os.Stdout.Fd()) && term.IsTerminal(int(os.Stdout.Fd()))
	if glint {
		glint = false
		if c, err := console.ConsoleFromFile(os.Stdout); err == nil {
			if sz, err := c.Size(); err == nil {
				glint = sz.Height > 0 && sz.Width > 0
			}
		}
	}

	if glint {
		return GlintUI(ctx)
	} else {
		return NonInteractiveUI(ctx)
	}
}

// Interpret decomposes the msg and arguments into the message, style, and writer
func Interpret(msg string, raw ...any) (string, string, io.Writer) {
	// Build our args and options
	var args []any
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
