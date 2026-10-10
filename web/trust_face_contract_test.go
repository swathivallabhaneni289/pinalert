package web

import (
	"strings"
	"testing"
)

// This file holds plan 03-22's contract for the trust toggle face (A77, A78,
// A79): the face wrapper and the chevron that createTrustBlock builds, the
// flex toggle with its hover fill and no underline, and the border cue under
// forced colours. It reuses the p13 helpers of trust_contract_test.go; every
// helper this file declares would start with p22. Honest limit: static
// inspection proves the shipped text and the declared CSS, not paint. The
// browser proof is run by the orchestrator after the merge.

// TestTrustToggleFaceMarkup pins the markup order of the toggle: the face
// wraps the counts and the signals, the chevron follows the face, the hidden
// hint comes last, the chevron is hidden from assistive technology and the
// toggle stays the block's only focus stop (A78, A79, D-20).
func TestTrustToggleFaceMarkup(t *testing.T) {
	// Code lines only, so a comment above the next function cannot count.
	create := jsFunctionBody(t, p13CodeLines(p13ReadStatic(t, "static/js/trust.js")), "createTrustBlock(options)")

	p13MustContain(t, "createTrustBlock", create,
		"'trust-block__face'",
		"'trust-block__chevron'",
		"chevron.setAttribute('aria-hidden', 'true')",
	)
	p13Ordered(t, create, "createTrustBlock toggle children",
		"face.appendChild(counts)",
		"face.appendChild(signals)",
		"toggle.appendChild(face)",
		"toggle.appendChild(chevron)",
		"toggle.appendChild(hint)",
	)
	p13MustNotContain(t, "createTrustBlock", create,
		"toggle.appendChild(counts)",
		"toggle.appendChild(signals)",
		"tabindex",
		"tabIndex",
	)
}

// TestTrustToggleFaceStyle pins the declared CSS of the face, the chevron,
// the hover and press fills, the reduced motion and forced colours rules, the
// underline ban and the glyph file (A77, A78, A79).
func TestTrustToggleFaceStyle(t *testing.T) {
	rules := p13ParseCSS(p13ReadStatic(t, "static/css/trust.css"))

	want := func(label string, r p13CSSRule, decls map[string]string) {
		t.Helper()
		for prop, value := range decls {
			if got, ok := r.Decls[prop]; !ok || got != value {
				t.Errorf("%s %s = %q, want %q", label, prop, got, value)
			}
		}
	}

	// The face basis is load bearing: the widest count plus a margin.
	face, _ := p13Rule(t, rules, "", ".trust-block__face:not([hidden])")
	want(".trust-block__face:not([hidden])", face, map[string]string{
		"display": "block", "flex": "1 1 6.85em", "min-width": "0",
	})

	chevronHead := ".trust-block__chevron:not([hidden])"
	chevron, chevronIdx := p13Rule(t, rules, "", chevronHead)
	chevronURL := `url("/static/icons/chevron-down.svg")`
	want(chevronHead, chevron, map[string]string{
		"display": "block", "flex": "none", "width": "20px", "height": "20px",
		"background-color":      "currentColor",
		"-webkit-mask-image":    chevronURL,
		"mask-image":            chevronURL,
		"-webkit-mask-size":     "contain",
		"mask-size":             "contain",
		"-webkit-mask-repeat":   "no-repeat",
		"mask-repeat":           "no-repeat",
		"-webkit-mask-position": "center",
		"mask-position":         "center",
		"transition":            "transform 150ms ease",
	})

	openHead := `.trust-block__toggle[aria-expanded="true"] .trust-block__chevron`
	open, openIdx := p13Rule(t, rules, "", openHead)
	want(openHead, open, map[string]string{"transform": "rotate(180deg)"})
	if openIdx <= chevronIdx {
		t.Errorf("the open chevron rule must come after the chevron base rule")
	}

	// Hover is a soft fill and nothing else; the press fill wins while held.
	hover, hoverIdx := p13Rule(t, rules, "", ".trust-block__toggle:hover")
	if len(hover.Decls) != 1 {
		t.Errorf("the toggle hover rule declares %d properties, want exactly 1: %v", len(hover.Decls), hover.Decls)
	}
	want(".trust-block__toggle:hover", hover, map[string]string{
		"background-color": "color-mix(in srgb, var(--color-border) 50%, transparent)",
	})
	_, activeIdx := p13Rule(t, rules, "", ".trust-block__toggle:active")
	if activeIdx <= hoverIdx {
		t.Errorf("the :active rule must come after the :hover rule, or the press fill never shows")
	}

	// Reduced motion removes the chevron turn.
	motion, motionIdx := p13Rule(t, rules, "prefers-reduced-motion: reduce", chevronHead)
	want("reduced motion "+chevronHead, motion, map[string]string{"transition": "none"})
	if motionIdx <= chevronIdx {
		t.Errorf("the reduced motion chevron rule must come after the chevron base rule")
	}

	// Forced colours replace the fills, so hover and press show a border.
	forcedChevron, _ := p13Rule(t, rules, "forced-colors: active", chevronHead)
	want("forced colours "+chevronHead, forcedChevron, map[string]string{
		"forced-color-adjust": "none", "background-color": "CanvasText",
	})
	forcedToggleHead := ".trust-block__toggle:not([hidden])"
	forcedToggle, forcedToggleIdx := p13Rule(t, rules, "forced-colors: active", forcedToggleHead)
	want("forced colours "+forcedToggleHead, forcedToggle, map[string]string{"border": "1px solid Canvas"})
	forcedHover, forcedHoverIdx := p13Rule(t, rules, "forced-colors: active",
		".trust-block__toggle:hover, .trust-block__toggle:active")
	want("forced colours hover and press", forcedHover, map[string]string{"border-color": "CanvasText"})
	if forcedHoverIdx <= forcedToggleIdx {
		t.Errorf("the forced colours hover and press rule must come after the forced colours toggle rule")
	}
	for label, r := range map[string]p13CSSRule{
		"forced colours chevron": forcedChevron,
		"forced colours toggle":  forcedToggle,
		"forced colours hover":   forcedHover,
	} {
		if _, ok := r.Decls["display"]; ok {
			t.Errorf("%s declares display, which a forced colours rule must not", label)
		}
	}

	// A77: no underline in any state, in any media.
	banned := []string{
		"text-underline-offset", "text-decoration-line", "text-decoration-thickness",
		"text-decoration-color", "text-decoration-style",
	}
	for _, r := range rules {
		if !strings.Contains(r.Head, ".trust-") {
			continue
		}
		for _, prop := range banned {
			if _, ok := r.Decls[prop]; ok {
				t.Errorf("%q declares %s, but nothing in the trust block is underlined (A77)", r.Head, prop)
			}
		}
		if v, ok := r.Decls["text-decoration"]; ok && v != "none" {
			t.Errorf("%q declares text-decoration %q, want none (A77)", r.Head, v)
		}
	}

	// A77 decision: the hover fill is not wrapped in a hover media query.
	for _, r := range rules {
		if strings.Contains(r.Media, "hover") && strings.Contains(r.Head, ".trust-block__toggle") {
			t.Errorf("%q sits inside %q, but the hover fill is not guarded by a hover query (A77)", r.Head, r.Media)
		}
	}

	// The glyph file (A78).
	glyph := p13ReadStatic(t, "static/icons/chevron-down.svg")
	p13MustContain(t, "chevron-down.svg", glyph,
		`class="lucide lucide-chevron-down"`,
		`d="m6 9 6 6 6-6"`,
		`aria-hidden="true"`,
		`focusable="false"`,
	)
}
