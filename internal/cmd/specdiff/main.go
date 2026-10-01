// Command specdiff compares this library with the machine-readable Telegram
// Bot API specification and reports types, fields, methods and parameters
// that are missing from the Go code.
//
// Usage (from the repository root):
//
//	go run ./internal/cmd/specdiff            # report against the latest spec
//	go run ./internal/cmd/specdiff -strict    # exit with status 1 on gaps
//	go run ./internal/cmd/specdiff -v         # also list extra fields/params
//	go run ./internal/cmd/specdiff -spec api.json
//
// The specification is the api.json published by
// https://github.com/PaulSonOfLars/telegram-bot-api-spec, which is generated
// from https://core.telegram.org/bots/api.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const defaultSpec = "https://raw.githubusercontent.com/PaulSonOfLars/telegram-bot-api-spec/main/api.json"

// Go names that differ from the Bot API names.
var typeAliases = map[string]string{
	"MessageId":                       "MessageID",
	"LoginUrl":                        "LoginURL",
	"InlineQueryResultGif":            "InlineQueryResultGIF",
	"InlineQueryResultMpeg4Gif":       "InlineQueryResultMPEG4GIF",
	"InlineQueryResultCachedGif":      "InlineQueryResultCachedGIF",
	"InlineQueryResultCachedMpeg4Gif": "InlineQueryResultCachedMPEG4GIF",
}

// Types that are not represented by a struct.
var skipTypes = map[string]bool{
	"InputFile": true, // RequestFileData
}

// Wire fields that are routed by a custom (Un)MarshalJSON instead of a tag.
var customFields = map[string][]string{
	"OwnedGift":      {"gift"},
	"RichBlock":      {"caption"},
	"InputRichBlock": {"caption"},
}

// Methods implemented directly on BotAPI rather than through a config.
var directMethods = map[string]bool{
	"getMe":          true,
	"getWebhookInfo": true,
}

type specField struct {
	Name     string   `json:"name"`
	Types    []string `json:"types"`
	Required bool     `json:"required"`
}

type specEntry struct {
	Name     string      `json:"name"`
	Fields   []specField `json:"fields"`
	Subtypes []string    `json:"subtypes"`
}

type spec struct {
	Version     string               `json:"version"`
	ReleaseDate string               `json:"release_date"`
	Methods     map[string]specEntry `json:"methods"`
	Types       map[string]specEntry `json:"types"`
}

type goField struct {
	name     string
	json     string
	typ      string
	embedded bool
}

type pkg struct {
	structs map[string][]goField
	aliases map[string]string   // type X = Y
	methods map[string]string   // config type -> Bot API method
	keys    map[string][]string // config type -> parameter keys
	calls   map[string][]string // config type -> fields whose params()/addTo() are called
	params  map[string]bool     // config types that define their own params()
	consts  map[string]string
}

func main() {
	specSource := flag.String("spec", defaultSpec, "URL or path of the Bot API api.json")
	dir := flag.String("dir", ".", "directory of the tgbotapi package")
	strict := flag.Bool("strict", false, "exit with status 1 if anything is missing")
	verbose := flag.Bool("v", false, "also report fields and parameters that are not in the spec")
	flag.Parse()

	s, err := loadSpec(*specSource)
	if err != nil {
		fmt.Fprintln(os.Stderr, "loading spec:", err)
		os.Exit(2)
	}

	p, err := loadPackage(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parsing package:", err)
		os.Exit(2)
	}

	missing, extra := compare(s, p)

	fmt.Printf("Telegram %s (%s)\n\n", s.Version, s.ReleaseDate)
	if len(missing) == 0 {
		fmt.Println("Nothing is missing.")
	} else {
		fmt.Printf("Missing (%d):\n", len(missing))
		for _, m := range missing {
			fmt.Println("  -", m)
		}
	}
	if *verbose && len(extra) > 0 {
		fmt.Printf("\nNot in the spec (%d):\n", len(extra))
		for _, e := range extra {
			fmt.Println("  -", e)
		}
	}

	if *strict && len(missing) > 0 {
		os.Exit(1)
	}
}

func loadSpec(source string) (*spec, error) {
	var r io.Reader
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		resp, err := http.Get(source)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("GET %s: %s", source, resp.Status)
		}
		r = resp.Body
	} else {
		f, err := os.Open(source)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r = f
	}

	var s spec
	if err := json.NewDecoder(r).Decode(&s); err != nil {
		return nil, err
	}

	return &s, nil
}

func exprString(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprString(t.X)
	case *ast.ArrayType:
		return "[]" + exprString(t.Elt)
	case *ast.SelectorExpr:
		return exprString(t.X) + "." + t.Sel.Name
	case *ast.MapType:
		return "map[" + exprString(t.Key) + "]" + exprString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	}
	return "?"
}

func loadPackage(dir string) (*pkg, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		return nil, err
	}

	p := &pkg{
		structs: map[string][]goField{},
		aliases: map[string]string{},
		methods: map[string]string{},
		keys:    map[string][]string{},
		calls:   map[string][]string{},
		params:  map[string]bool{},
		consts:  map[string]string{},
	}

	for _, astPkg := range pkgs {
		for _, f := range astPkg.Files {
			for _, decl := range f.Decls {
				switch d := decl.(type) {
				case *ast.GenDecl:
					p.addGenDecl(d)
				case *ast.FuncDecl:
					p.addFuncDecl(d)
				}
			}
		}
	}

	return p, nil
}

func (p *pkg) addGenDecl(d *ast.GenDecl) {
	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if s.Assign != 0 {
				p.aliases[s.Name.Name] = exprString(s.Type)
				continue
			}
			st, ok := s.Type.(*ast.StructType)
			if !ok {
				continue
			}
			var fields []goField
			for _, fl := range st.Fields.List {
				tag := ""
				if fl.Tag != nil {
					v, _ := strconv.Unquote(fl.Tag.Value)
					tag = strings.Split(reflect.StructTag(v).Get("json"), ",")[0]
				}
				typ := exprString(fl.Type)
				if len(fl.Names) == 0 {
					fields = append(fields, goField{name: strings.TrimPrefix(typ, "*"), json: tag, typ: typ, embedded: true})
				}
				for _, n := range fl.Names {
					fields = append(fields, goField{name: n.Name, json: tag, typ: typ})
				}
			}
			p.structs[s.Name.Name] = fields
		case *ast.ValueSpec:
			for i, n := range s.Names {
				if i >= len(s.Values) {
					continue
				}
				if lit, ok := s.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					p.consts[n.Name], _ = strconv.Unquote(lit.Value)
				}
			}
		}
	}
}

func (p *pkg) addFuncDecl(d *ast.FuncDecl) {
	if d.Recv == nil || len(d.Recv.List) == 0 || d.Body == nil {
		return
	}
	recv := strings.TrimPrefix(exprString(d.Recv.List[0].Type), "*")

	switch d.Name.Name {
	case "method":
		ast.Inspect(d.Body, func(n ast.Node) bool {
			ret, ok := n.(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 {
				return true
			}
			switch v := ret.Results[0].(type) {
			case *ast.BasicLit:
				p.methods[recv], _ = strconv.Unquote(v.Value)
			case *ast.Ident:
				p.methods[recv] = "$" + v.Name
			}
			return true
		})
	case "params", "addTo":
		ast.Inspect(d.Body, func(n ast.Node) bool {
			switch c := n.(type) {
			case *ast.CallExpr:
				sel, ok := c.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if sel.Sel.Name == "params" || sel.Sel.Name == "addTo" {
					if inner, ok := sel.X.(*ast.SelectorExpr); ok {
						p.calls[recv] = append(p.calls[recv], inner.Sel.Name)
					}
					return true
				}
				if len(c.Args) > 0 {
					if lit, ok := c.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						key, _ := strconv.Unquote(lit.Value)
						p.keys[recv] = append(p.keys[recv], key)
					}
				}
			case *ast.IndexExpr:
				if lit, ok := c.Index.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					key, _ := strconv.Unquote(lit.Value)
					p.keys[recv] = append(p.keys[recv], key)
				}
			}
			return true
		})
		if d.Name.Name == "params" {
			p.params[recv] = true
		}
	case "files":
		ast.Inspect(d.Body, func(n ast.Node) bool {
			kv, ok := n.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			if id, ok := kv.Key.(*ast.Ident); ok && id.Name == "Name" {
				if lit, ok := kv.Value.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					key, _ := strconv.Unquote(lit.Value)
					p.keys[recv] = append(p.keys[recv], key)
				}
			}
			return true
		})
	}
}

func (p *pkg) resolve(name string) string {
	for {
		target, ok := p.aliases[name]
		if !ok {
			return name
		}
		name = target
	}
}

// jsonFields returns the wire fields of a struct, including embedded ones.
func (p *pkg) jsonFields(name string, seen map[string]bool) map[string]goField {
	name = p.resolve(name)
	out := map[string]goField{}
	if seen[name] {
		return out
	}
	seen[name] = true

	for _, f := range p.structs[name] {
		switch {
		case f.embedded && f.json == "":
			for k, v := range p.jsonFields(f.name, seen) {
				out[k] = v
			}
		case f.json != "" && f.json != "-":
			out[f.json] = f
		}
	}
	for _, k := range customFields[name] {
		out[k] = goField{name: k, json: k, typ: "custom"}
	}

	return out
}

// paramKeys returns the parameter keys sent by a config, following calls to
// the params() of embedded base configs.
func (p *pkg) paramKeys(cfg string, seen map[string]bool) map[string]bool {
	cfg = p.resolve(cfg)
	out := map[string]bool{}
	if seen[cfg] {
		return out
	}
	seen[cfg] = true

	for _, k := range p.keys[cfg] {
		out[k] = true
	}

	fieldTypes := map[string]string{}
	var embedded []string
	for _, f := range p.structs[cfg] {
		fieldTypes[f.name] = strings.TrimPrefix(f.typ, "*")
		if f.embedded {
			embedded = append(embedded, f.name)
		}
	}

	calls := p.calls[cfg]
	if !p.params[cfg] {
		// params() is promoted from the embedded structs.
		calls = append(calls, embedded...)
	}
	for _, c := range calls {
		typ, ok := fieldTypes[c]
		if !ok {
			typ = c
		}
		for k := range p.paramKeys(typ, seen) {
			out[k] = true
		}
	}

	return out
}

var arrayOf = regexp.MustCompile(`^Array of (.+)$`)

func (p *pkg) goTypeFor(specType string) string {
	if m := arrayOf.FindStringSubmatch(specType); m != nil {
		return "[]" + p.goTypeFor(m[1])
	}
	switch specType {
	case "Integer":
		return "int"
	case "String":
		return "string"
	case "Boolean", "True":
		return "bool"
	case "Float":
		return "float64"
	case "InputFile":
		return "RequestFileData"
	}
	if alias, ok := typeAliases[specType]; ok {
		return alias
	}
	return specType
}

func normalize(t string) string {
	t = strings.TrimPrefix(t, "*")
	t = strings.ReplaceAll(t, "[]*", "[]")
	t = strings.ReplaceAll(t, "int64", "int")
	return t
}

func (p *pkg) typeMatches(specTypes []string, goType string) bool {
	g := normalize(goType)
	switch g {
	case "interface{}", "any", "json.RawMessage", "custom":
		return true
	}
	for _, st := range specTypes {
		if normalize(p.goTypeFor(st)) == g {
			return true
		}
		// Files are always represented as RequestFileData, and a message that
		// may be inaccessible is represented as a Message.
		if st == "String" && g == "RequestFileData" {
			return true
		}
		if st == "MaybeInaccessibleMessage" && g == "Message" {
			return true
		}
	}
	return false
}

func compare(s *spec, p *pkg) (missing, extra []string) {
	// Union types are represented either as a single flat struct with a type
	// discriminator, or as one struct per variant.
	covered := map[string]bool{}
	var markCovered func(string)
	markCovered = func(name string) {
		covered[name] = true
		for _, sub := range s.Types[name].Subtypes {
			markCovered(sub)
		}
	}

	unionFields := func(name string) map[string][]specField {
		out := map[string][]specField{}
		var walk func(string)
		walk = func(n string) {
			t, ok := s.Types[n]
			if !ok {
				return
			}
			for _, sub := range t.Subtypes {
				walk(sub)
			}
			for _, f := range t.Fields {
				out[f.Name] = append(out[f.Name], f)
			}
		}
		walk(name)
		return out
	}

	for _, name := range sortedKeys(s.Types) {
		t := s.Types[name]
		goName := p.goTypeFor(name)
		if len(t.Subtypes) == 0 || len(p.structs[goName]) == 0 {
			continue
		}
		markCovered(name)
		have := p.jsonFields(goName, map[string]bool{})
		want := unionFields(name)
		for _, k := range sortedKeys(want) {
			f, ok := have[k]
			if !ok {
				missing = append(missing, fmt.Sprintf("field %s.%s", name, k))
				continue
			}
			var types []string
			for _, sf := range want[k] {
				types = append(types, sf.Types...)
			}
			if !p.typeMatches(types, f.typ) {
				missing = append(missing, fmt.Sprintf("field %s.%s has type %s, want %s", name, k, f.typ, strings.Join(types, " or ")))
			}
		}
		for _, k := range sortedKeys(have) {
			if _, ok := want[k]; !ok {
				extra = append(extra, fmt.Sprintf("field %s.%s", name, k))
			}
		}
	}

	for _, name := range sortedKeys(s.Types) {
		t := s.Types[name]
		if skipTypes[name] || len(t.Subtypes) > 0 {
			continue
		}
		goName := p.goTypeFor(name)
		if _, ok := p.structs[goName]; !ok {
			if !covered[name] {
				missing = append(missing, "type "+name)
			}
			continue
		}
		have := p.jsonFields(goName, map[string]bool{})
		want := map[string]bool{}
		for _, f := range t.Fields {
			want[f.Name] = true
			gf, ok := have[f.Name]
			if !ok {
				missing = append(missing, fmt.Sprintf("field %s.%s", name, f.Name))
				continue
			}
			if !p.typeMatches(f.Types, gf.typ) {
				missing = append(missing, fmt.Sprintf("field %s.%s has type %s, want %s", name, f.Name, gf.typ, strings.Join(f.Types, " or ")))
			}
		}
		for _, k := range sortedKeys(have) {
			if !want[k] {
				extra = append(extra, fmt.Sprintf("field %s.%s", name, k))
			}
		}
	}

	configs := map[string][]string{}
	for cfg, method := range p.methods {
		if strings.HasPrefix(method, "$") {
			method = p.consts[method[1:]]
		}
		configs[method] = append(configs[method], cfg)
	}

	for _, name := range sortedKeys(s.Methods) {
		cfgs := configs[name]
		if len(cfgs) == 0 {
			if !directMethods[name] {
				missing = append(missing, "method "+name)
			}
			continue
		}
		sort.Strings(cfgs)
		want := map[string]bool{}
		for _, f := range s.Methods[name].Fields {
			want[f.Name] = true
		}
		for _, cfg := range cfgs {
			have := p.paramKeys(cfg, map[string]bool{})
			for _, k := range sortedKeys(want) {
				if !have[k] {
					missing = append(missing, fmt.Sprintf("parameter %s(%s) in %s", name, k, cfg))
				}
			}
			for _, k := range sortedKeys(have) {
				if !want[k] {
					extra = append(extra, fmt.Sprintf("parameter %s(%s) in %s", name, k, cfg))
				}
			}
		}
	}

	for _, method := range sortedKeys(configs) {
		if _, ok := s.Methods[method]; !ok {
			missing = append(missing, fmt.Sprintf("method %s used by %s is not in the spec", method, strings.Join(configs[method], ", ")))
		}
	}

	return missing, extra
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
