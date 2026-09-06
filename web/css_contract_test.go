package web

import (
	"io/fs"
	"strings"
	"testing"
)

// stripCSSComments removes every /* ... */ block comment from a CSS source
// string. It works by repeatedly locating the next comment-open delimiter
// and the matching comment-close delimiter that follows it, then dropping
// that span. If a comment is left unterminated (no closing "*/"), the
// remainder of the string from the comment-open delimiter onward is
// dropped, since anything after an unterminated comment is unparseable
// anyway.
func stripCSSComments(css string) string {
	var b strings.Builder
	rest := css
	for {
		openIdx := strings.Index(rest, "/*")
		if openIdx == -1 {
			b.WriteString(rest)
			break
		}
		b.WriteString(rest[:openIdx])
		afterOpen := rest[openIdx+2:]
		closeIdx := strings.Index(afterOpen, "*/")
		if closeIdx == -1 {
			// Unterminated comment — drop the remainder entirely.
			break
		}
		rest = afterOpen[closeIdx+2:]
	}
	return b.String()
}

// TestModalBackdropHiddenGuard guards the UAT Test 1 blocker: an unguarded
// `display` declaration on the full-viewport `.modal-backdrop` scrim makes
// the entire primary map UI unreachable on load, because the browser's
// native `[hidden] { display: none }` rule is user-agent-origin and loses to
// an author-origin `display` declaration at equal specificity. This test
// fails the build if any shipped stylesheet ever sets `display` on the
// modal backdrop selector without a `:not([hidden])` guard.
func TestModalBackdropHiddenGuard(t *testing.T) {
	const backdropSelector = ".modal-backdrop"
	const guard = ":not([hidden])"

	var sawGuardedBackdropInMainCSS bool

	err := fs.WalkDir(StaticFS, "static/css", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".css") {
			return nil
		}

		raw, err := fs.ReadFile(StaticFS, path)
		if err != nil {
			return err
		}

		// Strip comments first so a rule's own explanatory comment can
		// mention the selector name without tripping the check below.
		text := stripCSSComments(string(raw))

		// Split on the closing-brace character to get one chunk per rule.
		chunks := strings.Split(text, "}")
		for _, chunk := range chunks {
			// Take everything after the LAST opening-brace character as the
			// declaration body, and everything before it as the selector
			// head. Splitting on the last brace (not the first) keeps a
			// rule nested inside an at-rule (e.g. a media query) parsing
			// correctly instead of mistaking the at-rule preamble for the
			// selector.
			lastOpen := strings.LastIndex(chunk, "{")
			if lastOpen == -1 {
				continue
			}
			selectorHead := chunk[:lastOpen]
			declBody := chunk[lastOpen+1:]

			if !strings.Contains(selectorHead, backdropSelector) {
				continue
			}
			if !strings.Contains(declBody, "display") {
				continue
			}

			if strings.Contains(selectorHead, guard) {
				if path == "static/css/main.css" {
					sawGuardedBackdropInMainCSS = true
				}
				continue
			}

			t.Errorf(
				"%s: found a rule setting `display` on the modal backdrop selector without a %q guard — selector head: %q",
				path, guard, strings.TrimSpace(selectorHead),
			)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk embedded static/css: %v", err)
	}

	if !sawGuardedBackdropInMainCSS {
		t.Fatalf("expected static/css/main.css to contain at least one guarded %s%s selector — found none (the rule may have been deleted outright)", backdropSelector, guard)
	}
}
