package handlers

// gofmt_test.go pins IN-E05 of the Phase 9 code review (2026-10-30): the
// handlers package stays gofmt clean. CI does not run gofmt, so this keeps a
// lost alignment space from going unnoticed.

import (
	"bytes"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandlerSourcesAreGofmtClean(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("listing the handler sources: %v (%d files)", err, len(files))
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue // Phase 9 changed no test file's formatting; sources only.
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading %s: %v", f, err)
		}
		out, err := format.Source(src)
		if err != nil {
			t.Errorf("%s does not parse: %v", f, err)
			continue
		}
		if !bytes.Equal(src, out) {
			t.Errorf("%s is not gofmt clean", f)
		}
	}
}
