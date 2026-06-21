// Package args provides an argparse-style command-line argument parser for
// Starlark scripts run by starcli. The surface deliberately mirrors Python's
// argparse so it reads familiarly:
//
//	load("args", "ArgumentParser")
//	p = ArgumentParser(description = "greet someone")
//	p.add_argument("--name", default = "World", help = "who to greet")
//	p.add_argument("--count", type = int, default = 1)
//	p.add_argument("--shout", action = "store_true")
//	p.add_argument("file", help = "input file")
//	ns = p.parse_args()            # parses the script's args (argv[1:])
//	print(ns.name, ns.count, ns.shout, ns.file)
//
// args.argv is the full argument vector (argv[0] is the script name or "-c",
// like Python's sys.argv); parse_args defaults to argv[1:].
package args

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ModuleName defines the module name.
const ModuleName = "args"

// NewModule creates the args module loader, capturing the script argv.
func NewModule(argv []string) starlet.ModuleLoader {
	av := make([]starlark.Value, len(argv))
	for i, a := range argv {
		av[i] = starlark.String(a)
	}
	sd := starlark.StringDict{
		"argv":           starlark.NewList(av),
		"ArgumentParser": starlark.NewBuiltin(ModuleName+".ArgumentParser", newParserBuiltin(argv)),
	}
	return dataconv.WrapModuleData(ModuleName, sd)
}

func newParserBuiltin(argv []string) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var prog, description string
		if err := starlark.UnpackArgs(b.Name(), args, kwargs, "prog?", &prog, "description?", &description); err != nil {
			return nil, err
		}
		if prog == "" {
			prog = "starcli"
		}
		return &parser{prog: prog, description: description, argv: argv}, nil
	}
}

type argSpec struct {
	name      string         // as declared, e.g. "--name" or "file"
	dest      string         // attribute name, e.g. "name" or "file"
	isOpt     bool           // declared with a leading dash
	typ       string         // "str" | "int" | "float" | "bool"
	storeTrue bool           // action="store_true": a valueless boolean flag
	def       starlark.Value // default value (starlark.None if unset)
	required  bool
	help      string
}

// parser is a Starlark value with add_argument / parse_args / format_help
// methods, mirroring argparse.ArgumentParser.
type parser struct {
	prog        string
	description string
	argv        []string
	specs       []*argSpec
	frozen      bool
}

var (
	_ starlark.Value    = (*parser)(nil)
	_ starlark.HasAttrs = (*parser)(nil)
)

func (p *parser) String() string        { return fmt.Sprintf("ArgumentParser(prog=%q)", p.prog) }
func (p *parser) Type() string          { return "ArgumentParser" }
func (p *parser) Freeze()               { p.frozen = true }
func (p *parser) Truth() starlark.Bool  { return starlark.True }
func (p *parser) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: ArgumentParser") }

func (p *parser) AttrNames() []string { return []string{"add_argument", "format_help", "parse_args"} }

func (p *parser) Attr(name string) (starlark.Value, error) {
	switch name {
	case "add_argument":
		return starlark.NewBuiltin("ArgumentParser.add_argument", p.addArgument), nil
	case "parse_args":
		return starlark.NewBuiltin("ArgumentParser.parse_args", p.parseArgs), nil
	case "format_help":
		return starlark.NewBuiltin("ArgumentParser.format_help", p.formatHelpBuiltin), nil
	}
	return nil, nil
}

func (p *parser) addArgument(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if p.frozen {
		return nil, fmt.Errorf("add_argument: parser is frozen")
	}
	var (
		name     string
		typeVal  starlark.Value = starlark.None
		def      starlark.Value = starlark.None
		help     string
		required bool
		action   string
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"name", &name, "type?", &typeVal, "default?", &def, "help?", &help, "required?", &required, "action?", &action); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("add_argument: name must not be empty")
	}

	typ, err := normalizeType(typeVal)
	if err != nil {
		return nil, err
	}
	spec := &argSpec{
		name:     name,
		isOpt:    strings.HasPrefix(name, "-"),
		dest:     strings.ReplaceAll(strings.TrimLeft(name, "-"), "-", "_"),
		typ:      typ,
		def:      def,
		help:     help,
		required: required,
	}
	switch action {
	case "":
		// value-taking argument
	case "store_true":
		spec.storeTrue = true
		spec.typ = "bool"
		if def == starlark.None {
			spec.def = starlark.False
		}
	default:
		return nil, fmt.Errorf("add_argument: unknown action %q (only store_true)", action)
	}
	p.specs = append(p.specs, spec)
	return starlark.None, nil
}

// normalizeType maps a type argument (Python-style `int`/`float`/`str`/`bool`
// builtins, or the equivalent string) to an internal type name.
func normalizeType(v starlark.Value) (string, error) {
	switch tv := v.(type) {
	case starlark.NoneType:
		return "str", nil
	case starlark.String:
		s := string(tv)
		if validType(s) {
			return s, nil
		}
	case *starlark.Builtin:
		if validType(tv.Name()) {
			return tv.Name(), nil
		}
	}
	return "", fmt.Errorf("add_argument: type must be int, float, str or bool")
}

func validType(s string) bool {
	switch s {
	case "str", "int", "float", "bool":
		return true
	}
	return false
}

func (p *parser) parseArgs(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var argvVal starlark.Value = starlark.None
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "argv?", &argvVal); err != nil {
		return nil, err
	}

	var toks []string
	if argvVal != starlark.None {
		iter := starlark.Iterate(argvVal)
		if iter == nil {
			return nil, fmt.Errorf("parse_args: argv must be an iterable of strings")
		}
		defer iter.Done()
		var x starlark.Value
		for iter.Next(&x) {
			s, ok := starlark.AsString(x)
			if !ok {
				return nil, fmt.Errorf("parse_args: argv elements must be strings, got %s", x.Type())
			}
			toks = append(toks, s)
		}
	} else if len(p.argv) > 1 {
		toks = p.argv[1:] // default: the script's args, like argparse on sys.argv[1:]
	}

	return p.parse(toks)
}

func (p *parser) parse(toks []string) (starlark.Value, error) {
	// index options by their declared name; collect positionals in order
	opts := map[string]*argSpec{}
	var positionals []*argSpec
	result := starlark.StringDict{}
	for _, s := range p.specs {
		result[s.dest] = s.def
		if s.isOpt {
			opts[s.name] = s
		} else {
			positionals = append(positionals, s)
		}
	}

	seen := map[string]bool{}
	posIdx := 0
	endOpts := false
	for i := 0; i < len(toks); i++ {
		tok := toks[i]
		switch {
		case tok == "--" && !endOpts:
			endOpts = true
		case !endOpts && len(tok) > 1 && strings.HasPrefix(tok, "-"):
			name, inline, hasInline := tok, "", false
			if eq := strings.IndexByte(tok, '='); eq >= 0 {
				name, inline, hasInline = tok[:eq], tok[eq+1:], true
			}
			spec, ok := opts[name]
			if !ok {
				return nil, p.errorf("unrecognized argument: %s", name)
			}
			if spec.storeTrue {
				if hasInline {
					return nil, p.errorf("argument %s: takes no value", name)
				}
				result[spec.dest] = starlark.True
			} else {
				valStr := inline
				if !hasInline {
					i++
					if i >= len(toks) {
						return nil, p.errorf("argument %s: expected one value", name)
					}
					valStr = toks[i]
				}
				v, err := convert(valStr, spec.typ)
				if err != nil {
					return nil, p.errorf("argument %s: %v", name, err)
				}
				result[spec.dest] = v
			}
			seen[spec.dest] = true
		default:
			if posIdx >= len(positionals) {
				return nil, p.errorf("unexpected positional argument: %s", tok)
			}
			spec := positionals[posIdx]
			posIdx++
			v, err := convert(tok, spec.typ)
			if err != nil {
				return nil, p.errorf("argument %s: %v", spec.name, err)
			}
			result[spec.dest] = v
			seen[spec.dest] = true
		}
	}

	// required options
	for _, s := range p.specs {
		if s.isOpt && s.required && !seen[s.dest] {
			return nil, p.errorf("the following argument is required: %s", s.name)
		}
	}
	// positionals are required unless a default was provided
	for ; posIdx < len(positionals); posIdx++ {
		s := positionals[posIdx]
		if s.def == starlark.None {
			return nil, p.errorf("the following argument is required: %s", s.name)
		}
	}

	return starlarkstruct.FromStringDict(starlark.String("Namespace"), result), nil
}

func (p *parser) errorf(format string, a ...interface{}) error {
	return fmt.Errorf("%s: error: %s", p.prog, fmt.Sprintf(format, a...))
}

func convert(s, typ string) (starlark.Value, error) {
	switch typ {
	case "str":
		return starlark.String(s), nil
	case "int":
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid int value: %q", s)
		}
		return starlark.MakeInt64(n), nil
	case "float":
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float value: %q", s)
		}
		return starlark.Float(f), nil
	case "bool":
		switch strings.ToLower(s) {
		case "true", "1", "yes", "y":
			return starlark.True, nil
		case "false", "0", "no", "n":
			return starlark.False, nil
		}
		return nil, fmt.Errorf("invalid bool value: %q", s)
	}
	return nil, fmt.Errorf("unknown type: %s", typ)
}

func (p *parser) formatHelpBuiltin(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
		return nil, err
	}
	return starlark.String(p.formatHelp()), nil
}

func (p *parser) formatHelp() string {
	var opts, pos []*argSpec
	for _, s := range p.specs {
		if s.isOpt {
			opts = append(opts, s)
		} else {
			pos = append(pos, s)
		}
	}

	var usage strings.Builder
	fmt.Fprintf(&usage, "usage: %s", p.prog)
	for _, s := range opts {
		if s.storeTrue {
			fmt.Fprintf(&usage, " [%s]", s.name)
		} else {
			fmt.Fprintf(&usage, " [%s %s]", s.name, strings.ToUpper(s.dest))
		}
	}
	for _, s := range pos {
		fmt.Fprintf(&usage, " %s", s.dest)
	}
	usage.WriteByte('\n')

	if p.description != "" {
		fmt.Fprintf(&usage, "\n%s\n", p.description)
	}
	writeSection(&usage, "positional arguments", pos)
	writeSection(&usage, "options", opts)
	return usage.String()
}

func writeSection(w *strings.Builder, title string, specs []*argSpec) {
	if len(specs) == 0 {
		return
	}
	// stable order for deterministic help
	sorted := append([]*argSpec(nil), specs...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].name < sorted[j].name })
	fmt.Fprintf(w, "\n%s:\n", title)
	for _, s := range sorted {
		fmt.Fprintf(w, "  %-16s %s\n", s.name, s.help)
	}
}
