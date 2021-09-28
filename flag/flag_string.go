package flag

import (
	"os"

	"github.com/posener/complete"
)

// -- StringVar and stringValue
type StringVar struct {
	Name       string
	Aliases    []string
	Usage      string
	Default    string
	Hidden     bool
	EnvVar     string
	Target     *string
	Completion complete.Predictor
	SetHook    func(val string)
}

type StringVarP struct {
	*StringVar
	Shorthand string
}

func (f *Set) StringVar(i *StringVar) {
	f.StringVarP(&StringVarP{
		StringVar: i,
		Shorthand: "",
	})
}

func (f *Set) StringVarP(i *StringVarP) {
	initial := i.Default
	if v, exist := os.LookupEnv(i.EnvVar); exist {
		initial = v
	}

	def := ""
	if i.Default != "" {
		def = i.Default
	}

	f.VarFlagP(&VarFlagP{
		VarFlag: &VarFlag{
			Name:       i.Name,
			Aliases:    i.Aliases,
			Usage:      i.Usage,
			Default:    def,
			EnvVar:     i.EnvVar,
			Value:      newStringValue(i, initial, i.Target, i.Hidden),
			Completion: i.Completion,
		},
		Shorthand: i.Shorthand,
	})
}

type stringValue struct {
	v      *StringVarP
	hidden bool
	target *string
}

func newStringValue(v *StringVarP, def string, target *string, hidden bool) *stringValue {
	*target = def
	return &stringValue{
		v:      v,
		hidden: hidden,
		target: target,
	}
}

func (s *stringValue) Set(val string) error {
	*s.target = val

	if s.v.SetHook != nil {
		s.v.SetHook(val)
	}

	return nil
}

func (s *stringValue) Get() interface{} { return *s.target }
func (s *stringValue) String() string   { return *s.target }
func (s *stringValue) Example() string  { return "string" }
func (s *stringValue) Hidden() bool     { return s.hidden }
func (s *stringValue) Type() string     { return "string" }
