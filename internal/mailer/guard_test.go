package mailer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	mailerImportPath   = "pinalert/internal/mailer"
	smtptestImportPath = "pinalert/internal/mailer/smtptest"
	rootCAsConstructor = "NewSMTPMailerWithRootCAs"
)

// goListPackage is one line of the go list output this guard reads.
type goListPackage struct {
	importPath string
	dir        string
	files      []string // every Go file of the package that is not a test file
	imports    []string // what those files import
}

// goListPackages lists the packages under the module root with their non test
// files and their direct imports. go list skips directories that start with a
// dot, so worktrees and tool folders are not walked.
func goListPackages(t *testing.T, root string) []goListPackage {
	t.Helper()
	format := "{{.ImportPath}}\t{{.Dir}}\t{{join .GoFiles \",\"}},{{join .CgoFiles \",\"}},{{join .IgnoredGoFiles \",\"}}\t{{join .Imports \",\"}}"
	cmd := exec.Command("go", "list", "-f", format, "./...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list ./...: %v", err)
	}
	var pkgs []goListPackage
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		cols := strings.Split(line, "\t")
		if len(cols) != 4 {
			t.Fatalf("unexpected go list line: %q", line)
		}
		p := goListPackage{importPath: cols[0], dir: cols[1]}
		for _, f := range strings.Split(cols[2], ",") {
			if f != "" {
				p.files = append(p.files, f)
			}
		}
		for _, i := range strings.Split(cols[3], ",") {
			if i != "" {
				p.imports = append(p.imports, i)
			}
		}
		pkgs = append(pkgs, p)
	}
	return pkgs
}

// forbiddenMailerUses returns one line for each use of the two test only hooks
// outside internal/mailer: an import of smtptest, or any mention of
// NewSMTPMailerWithRootCAs in the code of a non test file (a comment does not
// count). A package inside internal/mailer, smtptest included, may use them.
func forbiddenMailerUses(pkgs []goListPackage) []string {
	var found []string
	for _, p := range pkgs {
		if p.importPath == mailerImportPath || strings.HasPrefix(p.importPath, mailerImportPath+"/") {
			continue
		}
		for _, imp := range p.imports {
			if imp == smtptestImportPath {
				found = append(found, p.importPath+" imports "+smtptestImportPath)
			}
		}
		for _, file := range p.files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			path := filepath.Join(p.dir, file)
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				found = append(found, path+" could not be parsed: "+err.Error())
				continue
			}
			for _, spec := range f.Imports {
				if strings.Trim(spec.Path.Value, `"`) == smtptestImportPath {
					found = append(found, path+" imports "+smtptestImportPath)
				}
			}
			ast.Inspect(f, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && id.Name == rootCAsConstructor {
					found = append(found, path+" mentions "+rootCAsConstructor)
				}
				return true
			})
		}
	}
	return found
}

// TestTestOnlyHooksStayOutOfProductionCode pins IN-03: the fake server package
// and the constructor that takes its own set of trusted roots are for tests.
// No non test file outside internal/mailer may import the first or call the
// second, so a later change cannot load a trust pool from the environment and
// weaken the certificate check.
func TestTestOnlyHooksStayOutOfProductionCode(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	pkgs := goListPackages(t, root)
	if len(pkgs) < 10 {
		t.Fatalf("go list found only %d packages under %s; the guard would check nothing", len(pkgs), root)
	}
	for _, line := range forbiddenMailerUses(pkgs) {
		t.Errorf("test only hook in production code: %s", line)
	}
}

// TestGuardFindsAViolation shows the guard is not vacuous: given a package
// outside internal/mailer that calls the constructor and imports smtptest, it
// reports both; a comment and a package inside internal/mailer are not
// reported.
func TestGuardFindsAViolation(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.go")
	src := "package bad\n\nimport (\n\t\"pinalert/internal/mailer\"\n\t\"pinalert/internal/mailer/smtptest\"\n)\n\nvar _ = smtptest.Options{}\nvar _ = mailer.NewSMTPMailerWithRootCAs\n"
	if err := os.WriteFile(bad, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	commentOnly := filepath.Join(dir, "comment.go")
	if err := os.WriteFile(commentOnly, []byte("package ok\n\n// NewSMTPMailerWithRootCAs is only named in this comment.\nvar x = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pkgs := []goListPackage{
		{importPath: "pinalert/cmd/bad", dir: dir, files: []string{"bad.go"}, imports: []string{smtptestImportPath}},
		{importPath: "pinalert/cmd/ok", dir: dir, files: []string{"comment.go"}},
		{importPath: "pinalert/internal/mailer/other", dir: dir, files: []string{"bad.go"}, imports: []string{smtptestImportPath}},
	}
	got := strings.Join(forbiddenMailerUses(pkgs), "\n")
	for _, want := range []string{"pinalert/cmd/bad imports " + smtptestImportPath, "bad.go imports " + smtptestImportPath, "bad.go mentions " + rootCAsConstructor} {
		if !strings.Contains(got, want) {
			t.Errorf("the guard did not report %q; it reported:\n%s", want, got)
		}
	}
	if strings.Contains(got, "comment.go") || strings.Contains(got, "mailer/other") {
		t.Errorf("the guard reported a comment or a package inside internal/mailer:\n%s", got)
	}
}
