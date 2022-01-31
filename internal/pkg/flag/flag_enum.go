package flag

import (
	"fmt"
	"os"
	"strings"

	"github.com/posener/complete"
)

// -- EnumVar and enumValue
type EnumVar struct {
	Name       string
	Aliases    []string
	Usage      string
	Values     []string
	Default    []string
	Hidden     bool
	EnvVar     string
	Target     *[]string
	Completion complete.Predictor
}

type EnumVarP struct {
	*EnumVar
	Shorthand string
}

func (f *Set) EnumVar(i *EnumVar) {
	f.EnumVarP(&EnumVarP{
		EnumVar:   i,
		Shorthand: "",
	})
}

func (f *Set) EnumVarP(i *EnumVarP) {
	initial := i.Default
	if v, exist := os.LookupEnv(i.EnvVar); exist {
		parts := strings.Split(v, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		initial = parts
	}

	def := ""
	if i.Default != nil {
		def = strings.Join(i.Default, ",")
	}

	possible := strings.Join(i.Values, ", ")

	f.VarFlagP(&VarFlagP{
		VarFlag: &VarFlag{
			Name:       i.Name,
			Aliases:    i.Aliases,
			Usage:      strings.TrimRight(i.Usage, ". \t") + ". One possible value from: " + possible + ".",
			Default:    def,
			EnvVar:     i.EnvVar,
			Value:      newEnumValue(i, initial, i.Target, i.Hidden),
			Completion: i.Completion,
		},
		Shorthand: i.Shorthand,
	})
}

type enumValue struct {
	ev     *EnumVarP
	hidden bool
	target *[]string
}

func newEnumValue(ev *EnumVarP, def []string, target *[]string, hidden bool) *enumValue {
	*target = def
	return &enumValue{
		ev:     ev,
		hidden: hidden,
		target: target,
	}
}

func (s *enumValue) Set(vals string) error {
	parts := strings.Split(vals, ",")

parts:
	for _, val := range parts {
		val = strings.TrimSpace(val)

		for _, p := range s.ev.Values {
			if p == val {
				*s.target = append(*s.target, strings.TrimSpace(val))
				continue parts
			}
		}

		return fmt.Errorf("'%s' not valid. Must be one of: %s", val, strings.Join(s.ev.Values, ", "))
	}

	return nil
}

func (s *enumValue) Get() interface{} { return *s.target }
func (s *enumValue) String() string   { return strings.Join(*s.target, ",") }
func (s *enumValue) Example() string  { return "string" }
func (s *enumValue) Hidden() bool     { return s.hidden }
func (s *enumValue) Type() string     { return "enum" }
