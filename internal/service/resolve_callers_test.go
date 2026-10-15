package service_test

// This file is the source guard for R-01 and R-02. Every function in the
// service package that calls Resolve must set a moderation state on a resolver
// input inside its own body: a Moderation key in a ReportMeta composite
// literal, or an assignment to a .Moderation selector. Otherwise a future read
// path would resolve a held or removed report as if it were clear and publish
// it. The scan reads the package's own source with the standard parser, so a
// comment that mentions Resolve, a string that holds the word and the Resolve
// declaration itself are never counted as callers.
//
// reliability.go is the one allowlisted file, by name. Phase 3's ClassifyOutcome
// makes one deliberate neutral Resolve call whose zero value moderation is
// intended: a report's outcome for the reporter rating is judged on the votes
// alone, and 04-29 makes the classifier neutral for withheld reports before that
// call without touching the call. The guard asserts that file still makes
// exactly one call, so a second one cannot hide behind the allowlist.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// resolveGuardScan parses one Go source file and reports how many Resolve calls
// its function bodies make and whether every function that calls Resolve sets
// moderation on a resolver input in its own body. setsMetaModeration is false
// when the file has no Resolve call at all.
func resolveGuardScan(name string, src []byte) (calls int, setsMetaModeration bool, err error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return 0, false, err
	}
	allSet := true
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		fnCalls := 0
		fnSets := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				if isResolveCall(x) {
					fnCalls++
				}
			case *ast.CompositeLit:
				if isReportMetaType(x.Type) && hasModerationKey(x) {
					fnSets = true
				}
			case *ast.AssignStmt:
				for _, lhs := range x.Lhs {
					if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "Moderation" {
						fnSets = true
					}
				}
			}
			return true
		})
		calls += fnCalls
		if fnCalls > 0 && !fnSets {
			allSet = false
		}
	}
	return calls, calls > 0 && allSet, nil
}

func isResolveCall(c *ast.CallExpr) bool {
	switch f := c.Fun.(type) {
	case *ast.Ident:
		return f.Name == "Resolve"
	case *ast.SelectorExpr:
		return f.Sel.Name == "Resolve"
	}
	return false
}

func isReportMetaType(e ast.Expr) bool {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name == "ReportMeta"
	case *ast.SelectorExpr:
		return t.Sel.Name == "ReportMeta"
	}
	return false
}

func hasModerationKey(lit *ast.CompositeLit) bool {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Moderation" {
			return true
		}
	}
	return false
}

func TestResolveCallersMentionModeration(t *testing.T) {
	synthetic := []struct {
		name      string
		src       string
		wantCalls int
		wantSets  bool
	}{
		{
			name:      "a comment mention is not a call",
			src:       "package p\n\n// Resolve(meta, tally, now) is described here.\nfunc f() {}\n",
			wantCalls: 0,
		},
		{
			name:      "a string literal is not a call",
			src:       "package p\n\nvar s = \"Resolve(meta, tally, now)\"\n",
			wantCalls: 0,
		},
		{
			name:      "the declaration is not a call",
			src:       "package p\n\nfunc Resolve(meta ReportMeta, t VoteTally, n int) int { return 0 }\n",
			wantCalls: 0,
		},
		{
			name:      "a call without moderation is caught",
			src:       "package p\n\nfunc f() {\n\tResolve(ReportMeta{Severity: s}, t, n)\n}\n",
			wantCalls: 1,
			wantSets:  false,
		},
		{
			name:      "a struct field named Moderation is not a resolver input",
			src:       "package p\n\ntype row struct{ Moderation string }\n\nfunc f() {\n\tResolve(ReportMeta{Severity: s}, t, n)\n}\n",
			wantCalls: 1,
			wantSets:  false,
		},
		{
			name:      "a Moderation key on another literal does not count",
			src:       "package p\n\nfunc f() {\n\t_ = ActivityReport{Moderation: m}\n\tResolve(ReportMeta{Severity: s}, t, n)\n}\n",
			wantCalls: 1,
			wantSets:  false,
		},
		{
			name:      "a Moderation key in a ReportMeta literal counts",
			src:       "package p\n\nfunc f() {\n\tResolve(ReportMeta{Severity: s, Moderation: m}, t, n)\n}\n",
			wantCalls: 1,
			wantSets:  true,
		},
		{
			name:      "an assignment to .Moderation counts",
			src:       "package p\n\nfunc f() {\n\tvar meta ReportMeta\n\tmeta.Moderation = m\n\tResolve(meta, t, n)\n}\n",
			wantCalls: 1,
			wantSets:  true,
		},
		{
			name:      "one function's literal never covers another function's call",
			src:       "package p\n\nfunc a() {\n\tResolve(ReportMeta{Severity: s, Moderation: m}, t, n)\n}\n\nfunc b() {\n\tResolve(ReportMeta{Severity: s}, t, n)\n}\n",
			wantCalls: 2,
			wantSets:  false,
		},
	}
	for _, c := range synthetic {
		t.Run(c.name, func(t *testing.T) {
			calls, sets, err := resolveGuardScan("synthetic.go", []byte(c.src))
			if err != nil {
				t.Fatalf("resolveGuardScan: %v", err)
			}
			if calls != c.wantCalls || sets != c.wantSets {
				t.Errorf("calls = %d, sets = %v, want %d and %v", calls, sets, c.wantCalls, c.wantSets)
			}
		})
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the package directory: %v", err)
	}
	callers := map[string]bool{}
	reliabilityCalls := -1
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		calls, sets, err := resolveGuardScan(name, src)
		if err != nil {
			t.Fatalf("scanning %s: %v", name, err)
		}
		if name == "reliability.go" {
			reliabilityCalls = calls
			continue
		}
		if calls > 0 {
			callers[name] = sets
		}
	}

	for _, want := range []string{"report.go", "trust.go", "auth.go", "historyread.go"} {
		if _, ok := callers[want]; !ok {
			t.Errorf("%s is not among the Resolve callers, so the guard may be scanning nothing", want)
		}
	}
	for name, sets := range callers {
		if !sets {
			t.Errorf("%s calls Resolve in a function that does not pass the report's moderation state: pass it into ReportMeta.Moderation (R-01)", name)
		}
	}
	if reliabilityCalls != 1 {
		t.Errorf("reliability.go makes %d Resolve calls, want exactly 1: Phase 3's single neutral call count changed", reliabilityCalls)
	}
}
