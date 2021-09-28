package flag

import (
	"bytes"
	"errors"
	goflag "flag"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	flag "github.com/spf13/pflag"

	"github.com/posener/complete"
)

// Sets is a group of flag sets.
type Sets struct {
	// unionSet is the set that is the union of all other sets. This
	// has ALL flags defined on it and is the set that is parsed. But
	// we maintain the other list of sets so that we can generate proper help.
	unionSet *flag.FlagSet

	// flagSets is the list of sets that we have. We don't parse these
	// directly but use them for help generation and autocompletion.
	flagSets []*Set

	// goflagSet has the same values as unionSet, but as std go flags
	// instead of posix flags. Used only as a fallback if parsing the
	// unionSet throws an error to support std go flag compatibility.
	goflagSet *goflag.FlagSet

	// completions is our set of autocompletion handlers. This is also
	// the union of all available flags similar to unionSet.
	completions complete.Flags
}

// NewSets creates a new flag sets.
func NewSets() *Sets {
	unionSet := flag.NewFlagSet("", flag.ContinueOnError)
	goflagSet := new(goflag.FlagSet)

	// Errors and usage are expected to be controlled externally by
	// checking on the result of Parse.
	// Do the same thing for both posix and std lib flags
	unionSet.Usage = func() {}
	unionSet.SetOutput(ioutil.Discard)

	goflagSet.Usage = func() {}
	goflagSet.SetOutput(ioutil.Discard)

	return &Sets{
		unionSet:    unionSet,
		completions: complete.Flags{},
		goflagSet:   goflagSet,
	}
}

// NewSet creates a new single flag set. A set should be created for
// any grouping of flags, for example "Common Options", "Auth Options", etc.
func (f *Sets) NewSet(name string) *Set {
	flagSet := NewSet(name)

	// The union and completions are pointers to our own values
	flagSet.unionSet = f.unionSet
	flagSet.completions = f.completions
	flagSet.goflagSet = f.goflagSet

	// Keep track of it for help generation
	f.flagSets = append(f.flagSets, flagSet)
	return flagSet
}

// Completions returns the completions for this flag set.
func (f *Sets) Completions() complete.Flags {
	return f.completions
}

// Parse parses the given flags, returning any errors.
// It does a naive check for std lib flags to determine which
// flag set to parse.
func (f *Sets) Parse(args []string) error {
	if hasGoFlags(args) {
		if err := f.goflagSet.Parse(args); err != nil {
			return err
		}

		// Std lib flags don't allow flags after positional args
		if err := checkFlagsAfterArgs(f.goflagSet.Args(), f); err != nil {
			return err
		}
		return nil
	}
	return f.unionSet.Parse(args)
}

// Parsed reports whether the command-line flags have been parsed.
func (f *Sets) Parsed() bool {
	return f.unionSet.Parsed()
}

// Args returns the remaining args after parsing.
func (f *Sets) Args() []string {
	// Check if parsing fell back to std go flags to return correct set of args
	if f.goflagSet.Parsed() {
		return f.goflagSet.Args()
	}
	return f.unionSet.Args()
}

// Returns whether the command is using the parsed goflag set or posix set
func (f *Sets) UsesGoflags() bool {
	return f.goflagSet.Parsed()
}

// Visit visits the flags in lexicographical order, calling fn for each. It
// visits only those flags that have been set.
func (f *Sets) Visit(fn func(*flag.Flag)) {
	f.unionSet.Visit(fn)
}

// Help builds custom help for this command, grouping by flag set.
func (fs *Sets) Help() string {
	var out bytes.Buffer

	for _, set := range fs.flagSets {
		printFlagTitle(&out, set.name+":")
		set.VisitAll(func(f *flag.Flag) {
			// Skip any hidden flags
			if f.Hidden {
				return
			}
			printFlagDetail(&out, f)
		})
	}

	return strings.TrimRight(out.String(), "\n")
}

// Help builds custom help for this command, grouping by flag set.
func (fs *Sets) VisitSets(fn func(name string, set *Set)) {
	for _, set := range fs.flagSets {
		fn(set.name, set)
	}
}

// TODO make this less horrendous my eyes are bleeding
func (fs *Sets) HideUnusedFlags(setName string, flagNames []string) {
	fs.VisitSets(func(name string, set *Set) {
		if set.name == setName {
			set.flagSet.VisitAll(func(flag *flag.Flag) {
				for _, flagName := range flagNames {
					if flag.Name == flagName {
						flag.Hidden = true
					}
				}
			})
		}
	})
}

// Set is a grouped wrapper around a real flag set and a grouped flag set.
type Set struct {
	name        string
	flagSet     *flag.FlagSet
	unionSet    *flag.FlagSet
	goflagSet   *goflag.FlagSet
	completions complete.Flags

	vars []*VarFlagP
}

// NewSet creates a new flag set.
func NewSet(name string) *Set {
	return &Set{
		name:    name,
		flagSet: flag.NewFlagSet(name, flag.ContinueOnError),
	}
}

// Name returns the name of this flag set.
func (f *Set) Name() string {
	return f.name
}

func (f *Set) Visit(fn func(*flag.Flag)) {
	f.flagSet.Visit(fn)
}

func (f *Set) VisitAll(fn func(*flag.Flag)) {
	f.flagSet.VisitAll(fn)
}

func (f *Set) VisitVars(fn func(*VarFlagP)) {
	for _, v := range f.vars {
		fn(v)
	}
}

// printFlagTitle prints a consistently-formatted title to the given writer.
func printFlagTitle(w io.Writer, s string) {
	fmt.Fprintf(w, "%s\n\n", s)
}

// printFlagDetail prints a single flag to the given writer.
func printFlagDetail(w io.Writer, f *flag.Flag) {
	// Check if the flag is hidden - do not print any flag detail or help output
	// if it is hidden.
	if h, ok := f.Value.(FlagVisibility); ok && h.Hidden() {
		return
	}

	// This section is copied from the pflag library, with some tweaks
	// to fit the internal pkg library
	if f.Shorthand != "" {
		fmt.Fprintf(w, "  -%s, --%s", f.Shorthand, f.Name)
	} else {
		fmt.Fprintf(w, "      --%s", f.Name)
	}

	// Check for an example type
	example := ""
	if t, ok := f.Value.(FlagExample); ok {
		example = t.Example()
	}

	if example != "" {
		fmt.Fprintf(w, "=<%s>", example)
	}

	if !defaultIsZeroValue(f) {
		if f.Value.Type() == "string" {
			fmt.Fprintf(w, " (default %q", f.DefValue)
		} else {
			fmt.Fprintf(w, " (default %s)", f.DefValue)
		}
	}
	if len(f.Deprecated) != 0 {
		fmt.Fprintf(w, " (DEPRECATED: %s)", f.Deprecated)
	}
	fmt.Fprint(w, "\n")

	usage := reRemoveWhitespace.ReplaceAllString(f.Usage, " ")
	indented := wrapAtLengthWithPadding(usage, 8)
	fmt.Fprintf(w, "%s\n\n", indented)
}

// Copied from pflag library. The only modifications are deleting types that
// don't exist in the nomad-pack internal flag pkg
// defaultIsZeroValue returns true if the default value for this flag represents
// a zero value.
// TODO will probably need to be improved to handle enums and other non-native
// pflag types
func defaultIsZeroValue(f *flag.Flag) bool {
	switch f.Value.(type) {
	case boolFlag:
		return f.DefValue == "false"
	case *durationValue:
		// Beginning in Go 1.7, duration zero values are "0s"
		return f.DefValue == "0" || f.DefValue == "0s"
	case *intValue, *int64Value, *uintValue, *uint64Value, *float64Value:
		return f.DefValue == "0"
	case *stringValue:
		return f.DefValue == ""
	case *stringSliceValue:
		return f.DefValue == "[]" || f.DefValue == ""
	case *stringMapValue:
		return f.DefValue == ""
	default:
		switch f.Value.String() {
		case "false":
			return true
		case "<nil>":
			return true
		case "":
			return true
		case "0":
			return true
		}
		return false
	}
}

// Copied directly from waypoint--we only use this check if we've already
// determined std lib flags are being used to provide more helpful error
// messages (otherwise the cli interprets -foo as an arg, not a flag).
// It only performs this check on defined flags to avoid false positives.
//
// checkFlagsAfterArgs checks for a very common user error scenario where
// CLI flags are specified after positional arguments. Since we use the
// stdlib flag package, this is not allowed. However, we can detect this
// scenario, and notify a user. We can't easily automatically fix it because
// its hard to tell positional vs intentional flags.
func checkFlagsAfterArgs(args []string, set *Sets) error {
	if len(args) == 0 {
		return nil
	}
	// Build up our arg map for easy searching.
	flagMap := map[string]struct{}{}
	for _, v := range args {
		// If we reach a "--" we're done. This is a common designator
		// in CLIs (such as exec) that everything following is fair game.
		if v == "--" {
			break
		}

		// There is always at least 2 chars in a flag "-v" example.
		if len(v) < 2 {
			continue
		}

		// Flags start with a hyphen
		if v[0] != '-' {
			continue
		}

		// Detect double hyphen flags too
		if v[1] == '-' {
			v = v[1:]
		}

		// More than double hyphen, ignore. note this looks like we can
		// go out of bounds and panic cause this is the 3rd char if we have
		// a double hyphen and we only protect on 2, but since we check first
		// against plain "--" we know that its not exactly "--" AND the length
		// is at least 2, meaning we can safely imply we have length 3+ for
		// double-hyphen prefixed values.
		if v[1] == '-' {
			continue
		}

		// If we have = for "-foo=bar", trim out the =.
		if idx := strings.Index(v, "="); idx >= 0 {
			v = v[:idx]
		}

		flagMap[v[1:]] = struct{}{}
	}

	// Now look for anything that looks like a flag we accept. We only
	// look for flags we accept because that is the most common error and
	// limits the false positives we'll get on arguments that want to be
	// hyphen-prefixed.
	didIt := false
	set.VisitSets(func(name string, s *Set) {
		s.VisitAll(func(f *flag.Flag) {
			if _, ok := flagMap[f.Name]; ok {
				// Uh oh, we done it. We put a flag after an arg.
				didIt = true
			}
		})
	})

	if didIt {
		return errFlagAfterArgs
	}

	return nil
}

// Very simple check of the flags to see if they're std lib flags or posix.
// Because posix flags use shorthands, this assumes that std lib flags will
// all be more than one char long, not including the flag (e.g. -p is posix,
// not std lib).
func hasGoFlags(args []string) bool {
	for _, arg := range args {
		// Ignore shorthands
		if len(arg) > 2 && arg[0] == '-' && arg[1] != '-' {
			return true
		}
	}
	return false
}

var errFlagAfterArgs = errors.New(strings.TrimSpace(`
Flags must be specified before positional arguments when using Go standard 
library style flags. For example, "nomad-pack plan -verbose example" instead
of "nomad-pack plan example -verbose".

The CLI also accepts posix flags, which does allow flags after positional
arguments. For example, both "nomad-pack plan --verbose example" and 
"nomad-pack plan example --verbose" are valid commands.`))
