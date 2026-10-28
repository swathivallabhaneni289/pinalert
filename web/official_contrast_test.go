package web

import "testing"

// TestOfficialLevelContrast pins the colour mapping of the Official alerts level
// chips (UI-SPEC Color, plan 05-09) in all four token sources, light and dark:
// the icon colour on its own tint and the text on every tint reach 4.5 to 1.
// The icon pairs use the tone tokens, not the severity borders, because
// --color-severity-medium on its light tint measures about 2.9 to 1. The six
// pairs are the ones the chips, the stale notice, the neutral chip and the
// authority badge and Demo tag actually draw.
func TestOfficialLevelContrast(t *testing.T) {
	needed := []string{
		"--color-bg", "--color-surface", "--color-text",
		"--color-severity-critical", "--color-severity-critical-bg", "--color-severity-medium-bg",
		"--color-tone-bad", "--color-tone-caution",
	}
	light1, light2, dark1, dark2 := extractThemeTokens(parseCSSRules(p408ReadStatic(t, "static/css/main.css")), needed)
	sources := []struct {
		name   string
		tokens themeTokens
	}{
		{"root light", light1}, {`data-theme="light"`, light2}, {"prefers dark", dark1}, {`data-theme="dark"`, dark2},
	}
	pairs := [][2]string{
		{"--color-tone-bad", "--color-severity-critical-bg"},   // Extreme icon on its tint
		{"--color-tone-caution", "--color-severity-medium-bg"}, // Severe icon on its tint
		{"--color-text", "--color-severity-critical-bg"},       // Extreme chip word
		{"--color-text", "--color-severity-medium-bg"},         // Severe chip word, stale notice text
		{"--color-text", "--color-surface"},                    // neutral chip, details panel
		{"--color-text", "--color-bg"},                         // badge, Demo tag, rows
	}
	for _, src := range sources {
		for _, p := range pairs {
			fg, okFg := resolveOfficialToken(src.tokens, p[0])
			bg, okBg := resolveOfficialToken(src.tokens, p[1])
			if !okFg || !okBg {
				t.Errorf("%s: %s or %s does not resolve", src.name, p[0], p[1])
				continue
			}
			ctx := src.name + " " + p[0] + " on " + p[1]
			if ratio := wcagContrastRatio(t, fg, bg, ctx); ratio < 4.5 {
				t.Errorf("%s: contrast %.2f is below 4.5", ctx, ratio)
			}
		}
	}
}

// resolveOfficialToken returns the hex value of a token, following one level of
// var(--other), which is how --color-tone-bad is declared in every theme block.
func resolveOfficialToken(tokens themeTokens, name string) (string, bool) {
	value, ok := tokens[name]
	if !ok {
		return "", false
	}
	if other, isVar := bareVarName(value); isVar {
		value, ok = tokens[other]
		if !ok {
			return "", false
		}
	}
	return value, true
}
