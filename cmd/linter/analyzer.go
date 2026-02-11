package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var Analyzer = &analysis.Analyzer{
	Name:     "exitcheck",
	Doc:      "reports panic usage and calls to os.Exit/log.Fatal* outside main.main of package main",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	var fnStack []ast.Node

	filter := []ast.Node{
		(*ast.FuncDecl)(nil),
		(*ast.FuncLit)(nil),
		(*ast.CallExpr)(nil),
	}

	insp.Nodes(filter, func(n ast.Node, push bool) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Body == nil {
				return true
			}
			if push {
				fnStack = append(fnStack, x)
			} else if len(fnStack) > 0 {
				fnStack = fnStack[:len(fnStack)-1]
			}

		case *ast.FuncLit:
			if push {
				fnStack = append(fnStack, x)
			} else if len(fnStack) > 0 {
				fnStack = fnStack[:len(fnStack)-1]
			}

		case *ast.CallExpr:
			if !push {
				return true
			}

			allowed := pass.Pkg != nil &&
				pass.Pkg.Name() == "main" &&
				len(fnStack) == 1 &&
				isMainDecl(fnStack[0])

			if isBuiltinPanic(pass, x) {
				pass.Reportf(x.Lparen, "panic is forbidden")
				return true
			}

			if isOsExit(pass, x) || isLogFatal(pass, x) {
				if !allowed {
					pass.Reportf(x.Lparen, "call to os.Exit or log.Fatal outside main function of main package")
				}
			}
		}
		return true
	})

	return nil, nil
}

func isMainDecl(n ast.Node) bool {
	fd, ok := n.(*ast.FuncDecl)
	if !ok {
		return false
	}
	return fd.Recv == nil && fd.Name != nil && fd.Name.Name == "main"
}

func isBuiltinPanic(pass *analysis.Pass, call *ast.CallExpr) bool {
	id, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	obj := pass.TypesInfo.Uses[id]
	b, ok := obj.(*types.Builtin)
	return ok && b.Name() == "panic"
}

func isOsExit(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	return ok && fn.Pkg() != nil && fn.Pkg().Path() == "os" && fn.Name() == "Exit"
}

func isLogFatal(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "log" {
		return false
	}

	switch fn.Name() {
	case "Fatal", "Fatalf", "Fatalln":
		return true
	default:
		return false
	}
}
