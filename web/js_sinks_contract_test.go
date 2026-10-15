package web

import (
	"io/fs"
	"strings"
	"testing"
)

// This file holds the one scan that proves no script under static/js parses a
// markup string (D-05, plan 04-08): report text and server messages reach the
// page as text only, so a link typed into a report stays inert on every
// surface, present and future. The scan walks the embedded static/js folder,
// so a script added by a later plan is covered the moment it exists.

const (
	jsStateCode = iota
	jsStateSingle
	jsStateDouble
	jsStateTemplate
)

// scanJSComments copies code and string literals (single quote, double quote
// and template literal, honouring backslash escapes and template expressions)
// and drops block comments and line comments. A "//" starts a line comment
// only outside a string and only at the start of a line or right after a
// whitespace character, so "https://x" and "a // b" inside strings survive.
// Newlines are kept so line numbers stay stable. The bool result is true when
// the scan ends inside an unterminated string, template literal or block
// comment, or meets a single or double quoted string that runs onto a new
// line; a regular expression literal holding a quote character looks exactly
// like that, and it would otherwise hide every later token in the file.
func scanJSComments(src string) (string, bool) {
	var out strings.Builder
	var braces []int // one open brace count per template expression in progress
	state := jsStateCode
	bad := false
	i := 0
	for i < len(src) {
		c := src[i]
		switch state {
		case jsStateCode:
			switch {
			case c == '\'':
				out.WriteByte(c)
				state = jsStateSingle
			case c == '"':
				out.WriteByte(c)
				state = jsStateDouble
			case c == '`':
				out.WriteByte(c)
				state = jsStateTemplate
			case c == '/' && i+1 < len(src) && src[i+1] == '*':
				body := src[i+2:]
				end := strings.Index(body, "*/")
				if end == -1 {
					bad = true
					i = len(src)
				} else {
					body = body[:end]
					i += 2 + end + 2
				}
				out.WriteString(strings.Repeat("\n", strings.Count(body, "\n")))
				continue
			case c == '/' && i+1 < len(src) && src[i+1] == '/' &&
				(i == 0 || src[i-1] == ' ' || src[i-1] == '\t' || src[i-1] == '\n' || src[i-1] == '\r'):
				for i < len(src) && src[i] != '\n' {
					i++
				}
				continue
			case c == '{' && len(braces) > 0:
				braces[len(braces)-1]++
				out.WriteByte(c)
			case c == '}' && len(braces) > 0:
				out.WriteByte(c)
				if braces[len(braces)-1] == 0 {
					braces = braces[:len(braces)-1]
					state = jsStateTemplate
				} else {
					braces[len(braces)-1]--
				}
			default:
				out.WriteByte(c)
			}
			i++
		case jsStateSingle, jsStateDouble:
			quote := byte('\'')
			if state == jsStateDouble {
				quote = '"'
			}
			out.WriteByte(c)
			if c == '\\' && i+1 < len(src) {
				out.WriteByte(src[i+1])
				i += 2
				continue
			}
			if c == quote {
				state = jsStateCode
			} else if c == '\n' {
				bad = true
				state = jsStateCode
			}
			i++
		case jsStateTemplate:
			if c == '\\' && i+1 < len(src) {
				out.WriteByte(c)
				out.WriteByte(src[i+1])
				i += 2
				continue
			}
			if c == '`' {
				out.WriteByte(c)
				state = jsStateCode
				i++
				continue
			}
			if c == '$' && i+1 < len(src) && src[i+1] == '{' {
				out.WriteString("${")
				braces = append(braces, 0)
				state = jsStateCode
				i += 2
				continue
			}
			out.WriteByte(c)
			i++
		}
	}
	if state != jsStateCode || len(braces) > 0 {
		bad = true
	}
	return out.String(), bad
}

// stripJSComments returns src with its comments removed (the first result of
// scanJSComments).
func stripJSComments(src string) string {
	out, _ := scanJSComments(src)
	return out
}

func TestStripJSComments(t *testing.T) {
	const sinkAssign = "el.innerHTML = value;"
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"block comment naming a sink is removed", "a(); /* el.innerHTML = 1 */ b();", "a();  b();"},
		{"multi line block comment keeps its newlines", "a();\n/* one\n two innerHTML */\nb();", "a();\n\n\nb();"},
		{"whole line comment is removed", "a();\n// el.innerHTML = 1\nb();", "a();\n\nb();"},
		{"indented whole line comment is removed", "a();\n    // outerHTML\nb();", "a();\n    \nb();"},
		{"trailing comment after code is removed", "a(); // insertAdjacentHTML note\nb();", "a(); \nb();"},
		{"single quoted url is kept", "var u = 'https://example.org/x';", "var u = 'https://example.org/x';"},
		{"double quoted string with spaced slashes is kept", `var s = "a // b";`, `var s = "a // b";`},
		{"template literal with slashes is kept", "var s = `a // b`;", "var s = `a // b`;"},
		{"escaped quote then slashes stays inside the string", `var s = 'it\'s // here';`, `var s = 'it\'s // here';`},
		{"template expression with a nested template is kept", "var s = `a ${ b ? `c` : '' } // d`;", "var s = `a ${ b ? `c` : '' } // d`;"},
		{"real sink assignment is kept so the scan finds it", sinkAssign, sinkAssign},
		{"sink after a comment line is kept", "// note\n" + sinkAssign, "\n" + sinkAssign},
	}
	for _, c := range cases {
		got, bad := scanJSComments(c.src)
		if bad {
			t.Errorf("%s: scan reported an unterminated construct", c.name)
		}
		if got != c.want {
			t.Errorf("%s:\n got  %q\n want %q", c.name, got, c.want)
		}
		if stripJSComments(c.src) != got {
			t.Errorf("%s: stripJSComments differs from scanJSComments", c.name)
		}
	}
	if !strings.Contains(stripJSComments("x = 1; el.innerHTML = 2;"), "innerHTML") {
		t.Error("code that assigns to a sink property must survive stripping")
	}

	for name, src := range map[string]string{
		"unterminated single quote string": "var s = 'abc\nvar t = 1;",
		"unterminated double quote string": "var s = \"abc",
		"unterminated template literal":    "var s = `abc",
		"unterminated block comment":       "a(); /* never closed",
		"regex literal holding a quote":    "var r = /[\"]/g;\nvar x = 1;",
	} {
		if _, bad := scanJSComments(src); !bad {
			t.Errorf("%s: scan did not report it as unterminated", name)
		}
	}
}

// minJSFilesScanned is the number of scripts under static/js when plan 04-08
// was built (13 at base 70a642b: Phase 3 and 3.1 added trust.js, map-tools.js,
// activity-tabs.js and visibility.js to the 10 present at 98aa346). The scan
// fails below it so it can never pass over an empty or shrunken folder.
const minJSFilesScanned = 13

func TestNoMarkupSinksAnywhere(t *testing.T) {
	sinks := []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write"}
	scanned := 0
	err := fs.WalkDir(StaticFS, "static/js", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".js") {
			return nil
		}
		raw, err := fs.ReadFile(StaticFS, path)
		if err != nil {
			return err
		}
		scanned++
		code, unterminated := scanJSComments(string(raw))
		if unterminated {
			t.Errorf("%s: the comment scan ended inside an unterminated string, template literal or block comment (a regular expression literal holding a quote character does this); fix the file or the scanner before trusting the sink check", path)
			return nil
		}
		for _, sink := range sinks {
			if strings.Contains(code, sink) {
				t.Errorf("%s: uses the markup parsing sink %s; report text must reach the page as text only (D-05)", path, sink)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking embedded static/js: %v", err)
	}
	if scanned < minJSFilesScanned {
		t.Errorf("scanned %d scripts under static/js, want at least %d", scanned, minJSFilesScanned)
	}
}
