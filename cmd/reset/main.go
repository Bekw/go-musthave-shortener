package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

const marker = "generate:reset"

func main() {
	root, err := findModuleRoot()
	if err != nil {
		fatal(err)
	}

	fset := token.NewFileSet()

	cfg := &packages.Config{
		Dir:  root,
		Fset: fset,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		fatal(err)
	}

	for _, p := range pkgs {
		for _, e := range p.Errors {
			fmt.Fprintf(os.Stderr, "packages load error: %s\n", e)
		}
	}

	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].PkgPath < pkgs[j].PkgPath })

	for _, pkg := range pkgs {
		if pkg.Types == nil || pkg.TypesInfo == nil || len(pkg.Syntax) == 0 || len(pkg.GoFiles) == 0 {
			continue
		}

		structs := findMarkedStructs(pkg)
		outPath := filepath.Join(packageDir(pkg), "reset_gen.go")

		if len(structs) == 0 {
			_ = os.Remove(outPath)
			continue
		}

		src, err := generateFile(pkg, structs)
		if err != nil {
			fatal(fmt.Errorf("generate for %s: %w", pkg.PkgPath, err))
		}

		if err := writeFileIfChanged(outPath, src); err != nil {
			fatal(fmt.Errorf("write %s: %w", outPath, err))
		}
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "resetgen:", err)
	os.Exit(1)
}

func findModuleRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("go.mod not found (запусти изнутри Go-модуля)")
}

func packageDir(pkg *packages.Package) string {
	return filepath.Dir(pkg.GoFiles[0])
}

func writeFileIfChanged(path string, data []byte) error {
	old, err := os.ReadFile(path)
	if err == nil && bytes.Equal(old, data) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

type fieldKind int

const (
	kindUnknown fieldKind = iota
	kindBasic
	kindSlice
	kindMap
	kindPointer
	kindInterface
	kindChan
	kindFunc
	kindOther
)

type structMeta struct {
	Name string
	Type *types.Named
	St   *types.Struct
}

type fieldMeta struct {
	Name         string
	Type         types.Type
	Kind         fieldKind
	Elem         types.Type
	ElemKind     fieldKind
	CanCallReset bool
}

func hasMarker(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	return strings.Contains(doc.Text(), marker)
}

func findMarkedStructs(pkg *packages.Package) []structMeta {
	typeNames := map[string]*types.TypeName{}

	for _, f := range pkg.Syntax {
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name == nil {
					continue
				}
				_, isStruct := ts.Type.(*ast.StructType)
				if !isStruct {
					continue
				}

				doc := ts.Doc
				if doc == nil {
					doc = gd.Doc
				}
				if !hasMarker(doc) {
					continue
				}

				obj := pkg.TypesInfo.Defs[ts.Name]
				tn, ok := obj.(*types.TypeName)
				if !ok || tn == nil {
					continue
				}
				typeNames[ts.Name.Name] = tn
			}
		}
	}

	if len(typeNames) == 0 {
		return nil
	}

	names := make([]string, 0, len(typeNames))
	for n := range typeNames {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]structMeta, 0, len(names))

	for _, n := range names {
		tn := typeNames[n]
		named, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}
		st, ok := named.Underlying().(*types.Struct)
		if !ok {
			continue
		}
		out = append(out, structMeta{
			Name: n,
			Type: named,
			St:   st,
		})
	}

	return out
}

type typeStringer struct {
	pkg         *types.Package
	pathToAlias map[string]string
	aliasToPath map[string]string
}

func newTypeStringer(pkg *types.Package) *typeStringer {
	return &typeStringer{
		pkg:         pkg,
		pathToAlias: map[string]string{},
		aliasToPath: map[string]string{},
	}
}

func (ts *typeStringer) qualifier(p *types.Package) string {
	if p == nil {
		return ""
	}
	if ts.pkg != nil && p.Path() == ts.pkg.Path() {
		return ""
	}

	path := p.Path()
	if a, ok := ts.pathToAlias[path]; ok {
		return a
	}

	base := p.Name()
	alias := base
	if isGoKeyword(alias) {
		alias = alias + "_"
	}

	if prev, ok := ts.aliasToPath[alias]; ok && prev != path {
		for i := 2; ; i++ {
			try := fmt.Sprintf("%s%d", alias, i)
			if _, exists := ts.aliasToPath[try]; !exists {
				alias = try
				break
			}
		}
	}

	ts.pathToAlias[path] = alias
	ts.aliasToPath[alias] = path
	return alias
}

func (ts *typeStringer) TypeString(t types.Type) string {
	return types.TypeString(t, ts.qualifier)
}

func (ts *typeStringer) Imports() (aliases []string, paths []string) {
	if len(ts.aliasToPath) == 0 {
		return nil, nil
	}
	type pair struct {
		alias string
		path  string
	}
	var list []pair
	for a, p := range ts.aliasToPath {
		list = append(list, pair{a, p})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].path == list[j].path {
			return list[i].alias < list[j].alias
		}
		return list[i].path < list[j].path
	})
	for _, it := range list {
		aliases = append(aliases, it.alias)
		paths = append(paths, it.path)
	}
	return aliases, paths
}

func isGoKeyword(s string) bool {
	switch s {
	case "break", "default", "func", "interface", "select",
		"case", "defer", "go", "map", "struct",
		"chan", "else", "goto", "package", "switch",
		"const", "fallthrough", "if", "range", "type",
		"continue", "for", "import", "return", "var":
		return true
	default:
		return false
	}
}

func classifyType(t types.Type) (fieldKind, types.Type, fieldKind) {
	if t == nil {
		return kindUnknown, nil, kindUnknown
	}

	switch u := t.Underlying().(type) {
	case *types.Basic:
		return kindBasic, nil, kindUnknown
	case *types.Slice:
		return kindSlice, nil, kindUnknown
	case *types.Map:
		return kindMap, nil, kindUnknown
	case *types.Pointer:
		ek, _, _ := classifyType(u.Elem())
		return kindPointer, u.Elem(), ek
	case *types.Interface:
		return kindInterface, nil, kindUnknown
	case *types.Chan:
		return kindChan, nil, kindUnknown
	case *types.Signature:
		return kindFunc, nil, kindUnknown
	default:
		return kindOther, nil, kindUnknown
	}
}

func canCallResetOnValueExpr(t types.Type) bool {
	if t == nil {
		return false
	}
	if hasMethodReset(t) {
		return true
	}
	if _, ok := t.(*types.Pointer); ok {
		return false
	}
	return hasMethodReset(types.NewPointer(t))
}

func hasMethodReset(t types.Type) bool {
	ms := types.NewMethodSet(t)
	for i := 0; i < ms.Len(); i++ {
		if ms.At(i).Obj().Name() == "Reset" {
			return true
		}
	}
	return false
}

func zeroBasicExpr(t types.Type) string {
	b, ok := t.Underlying().(*types.Basic)
	if !ok {
		return "0"
	}
	switch b.Kind() {
	case types.Bool:
		return "false"
	case types.String:
		return `""`
	case types.UnsafePointer:
		return "nil"
	default:
		return "0"
	}
}

func generateFile(pkg *packages.Package, structs []structMeta) ([]byte, error) {
	ts := newTypeStringer(pkg.Types)

	var body bytes.Buffer
	for _, sm := range structs {
		genResetMethod(&body, ts, sm)
	}

	var out bytes.Buffer
	out.WriteString("// Code generated by cmd/reset; DO NOT EDIT.\n")
	fmt.Fprintf(&out, "package %s\n\n", pkg.Name)

	aliases, paths := ts.Imports()
	if len(paths) > 0 {
		out.WriteString("import (\n")
		for i := range paths {
			fmt.Fprintf(&out, "\t%s %q\n", aliases[i], paths[i])
		}
		out.WriteString(")\n\n")
	}

	out.Write(body.Bytes())

	src, err := format.Source(out.Bytes())
	if err != nil {
		return out.Bytes(), fmt.Errorf("format: %w", err)
	}
	return src, nil
}

func genResetMethod(w *bytes.Buffer, ts *typeStringer, sm structMeta) {
	recvType := ts.TypeString(sm.Type)

	fmt.Fprintf(w, "func (rs *%s) Reset() {\n", recvType)
	w.WriteString("\tif rs == nil {\n\t\treturn\n\t}\n\n")

	st := sm.St
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if f == nil {
			continue
		}
		name := f.Name()
		if name == "_" {
			continue
		}

		kind, elem, elemKind := classifyType(f.Type())
		fm := fieldMeta{
			Name:         name,
			Type:         f.Type(),
			Kind:         kind,
			Elem:         elem,
			ElemKind:     elemKind,
			CanCallReset: canCallResetOnValueExpr(f.Type()),
		}

		genFieldReset(w, ts, fm)
	}

	w.WriteString("}\n\n")
}

func genFieldReset(w *bytes.Buffer, ts *typeStringer, f fieldMeta) {
	sel := "rs." + f.Name

	switch f.Kind {
	case kindSlice:
		fmt.Fprintf(w, "\t%s = %s[:0]\n", sel, sel)
		return
	case kindMap:
		fmt.Fprintf(w, "\tclear(%s)\n", sel)
		return
	case kindPointer:
		fmt.Fprintf(w, "\tif %s != nil {\n", sel)

		if canCallResetOnValueExpr(f.Type) {
			fmt.Fprintf(w, "\t\t%s.Reset()\n", sel)
			fmt.Fprintf(w, "\t}\n")
			return
		}

		switch f.ElemKind {
		case kindSlice:
			fmt.Fprintf(w, "\t\t*%s = (*%s)[:0]\n", sel, sel)
		case kindMap:
			fmt.Fprintf(w, "\t\tclear(*%s)\n", sel)
		case kindBasic:
			fmt.Fprintf(w, "\t\t*%s = %s\n", sel, zeroBasicExpr(f.Elem))
		case kindInterface, kindChan, kindFunc:
			fmt.Fprintf(w, "\t\t*%s = nil\n", sel)
		default:
			fmt.Fprintf(w, "\t\t*%s = *new(%s)\n", sel, ts.TypeString(f.Elem))
		}

		fmt.Fprintf(w, "\t}\n")
		return
	default:
		if f.CanCallReset {
			fmt.Fprintf(w, "\t%s.Reset()\n", sel)
			return
		}

		switch f.Kind {
		case kindBasic:
			fmt.Fprintf(w, "\t%s = %s\n", sel, zeroBasicExpr(f.Type))
		case kindInterface, kindChan, kindFunc:
			fmt.Fprintf(w, "\t%s = nil\n", sel)
		default:
			fmt.Fprintf(w, "\t%s = *new(%s)\n", sel, ts.TypeString(f.Type))
		}
	}
}
