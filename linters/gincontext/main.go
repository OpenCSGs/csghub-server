package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/types"
	"log"
	"os"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"
)

var lintPath = []string{
	"opencsg.com/csghub-server/api/handler",
}

func main() {
	tagPtr := flag.String("tags", "", "build tags")
	flag.Parse()

	cfg := &packages.Config{
		Mode:       packages.LoadAllSyntax,
		BuildFlags: []string{"-tags=" + *tagPtr},
	}

	initial, err := packages.Load(cfg, "./...")
	if err != nil {
		log.Fatal(err)
	}
	if len(initial) == 0 {
		log.Fatalf("no initial packages")
	}

	// Run analyzers (just one) on packages.
	analyzers := []*analysis.Analyzer{analyzer}
	graph, err := checker.Analyze(analyzers, initial, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = graph.PrintText(os.Stderr, -1)
	if err != nil {
		log.Fatal(err)
	}

	// Compute the exit code.
	var exitcode = 0
	graph.All()(func(act *checker.Action) bool {
		if len(act.Diagnostics) > 0 {
			exitcode = 1
		}
		return true
	})

	os.Exit(exitcode)
}

var analyzer = &analysis.Analyzer{
	Name: "gincontext",
	Doc:  "Find not converted gin context",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	if pass.Pkg == nil || !slices.Contains(lintPath, pass.Pkg.Path()) {
		return nil, nil
	}
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			ce, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if len(ce.Args) < 1 {
				return true
			}
			at := pass.TypesInfo.TypeOf(ce.Args[0])
			if !strings.HasSuffix(at.String(), "gin-gonic/gin.Context") {
				return true
			}
			caller := pass.TypesInfo.TypeOf(ce.Fun)
			csg, ok := caller.(*types.Signature)
			if !ok {
				return true
			}
			params := csg.Params()
			if params.Len() < 1 {
				return true
			}
			v := params.At(0)
			if v.Type().String() != "context.Context" {
				return true
			}

			newCtx := fmt.Sprintf("%s.Request.Context()", ce.Args[0].(*ast.Ident).Name)
			pass.Report(analysis.Diagnostic{
				Pos:     ce.Pos(),
				Message: "should use ctx.Request.Context",
				SuggestedFixes: []analysis.SuggestedFix{
					{
						Message: "should use gin request context",
						TextEdits: []analysis.TextEdit{
							{
								Pos:     ce.Args[0].Pos(),
								End:     ce.Args[0].End(),
								NewText: []byte(newCtx),
							},
						},
					},
				},
			})
			return true
		})
	}

	return nil, nil
}
