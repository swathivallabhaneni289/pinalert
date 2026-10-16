// This file holds the one emergency numbers setting of D-22 and the shared
// page chrome data of D-21 and D-24. Every number the unofficial bar and the
// later legal pages print comes from EmergencyNumbers, read once at startup,
// so a number is never typed into a template.

package handlers

import (
	"html/template"
	"strings"
)

// Environment variable names for the emergency numbers setting (D-22).
const (
	// EmergencyPrimaryEnv names the one number the bar tells people to call.
	EmergencyPrimaryEnv = "EMERGENCY_PRIMARY"
	// EmergencyOlderEnv names the comma list of older numbers. Set but empty
	// means none, so the bar drops the whole parenthetical.
	EmergencyOlderEnv = "EMERGENCY_OLDER"
	// EmergencyStateLinesEnv names the comma pair of state and district
	// helplines. Set but empty drops both.
	EmergencyStateLinesEnv = "EMERGENCY_STATE_LINES"
)

const (
	defaultEmergencyPrimary  = "112"
	defaultEmergencyStateRow = "1070"
	defaultEmergencyDistrict = "1077"
)

// EmergencyNumbers is the one numbers setting. Every method has a value
// receiver because templates call them on a value copy of the view model, and
// a pointer receiver method fails at execute time.
type EmergencyNumbers struct {
	Primary          string
	Older            []string
	StateHelpline    string
	DistrictHelpline string
}

// OlderList joins the older numbers with a comma and one space.
func (n EmergencyNumbers) OlderList() string {
	return strings.Join(n.Older, ", ")
}

// HasOlder reports whether any older number is configured.
func (n EmergencyNumbers) HasOlder() bool {
	return len(n.Older) > 0
}

// HasStateLines is true only when both helplines are set, so a half pair
// never renders half a sentence on the Disclaimer page.
func (n EmergencyNumbers) HasStateLines() bool {
	return n.StateHelpline != "" && n.DistrictHelpline != ""
}

// TelHref returns the telephone link target for the primary number. The type
// is template.URL because html/template rewrites a plain string carrying the
// telephone scheme in a URL attribute to #ZgotmplZ, while the typed value
// renders as written. That is safe here: the scheme is a fixed literal and
// Primary holds only digits (isEmergencyNumber in LoadEmergencyNumbers), and
// html/template still escapes the value for the attribute. The Disclaimer page
// uses it for its one tap to call link; the bar never does.
func (n EmergencyNumbers) TelHref() template.URL {
	return template.URL("tel:" + n.Primary)
}

// DefaultEmergencyNumbers returns the shipped numbers. Older is a new slice on
// every call so one caller appending to it never changes the next.
func DefaultEmergencyNumbers() EmergencyNumbers {
	return EmergencyNumbers{
		Primary:          defaultEmergencyPrimary,
		Older:            []string{"100", "101", "108"},
		StateHelpline:    defaultEmergencyStateRow,
		DistrictHelpline: defaultEmergencyDistrict,
	}
}

// isEmergencyNumber accepts 2 to 15 ASCII digits and nothing else.
func isEmergencyNumber(s string) bool {
	if len(s) < 2 || len(s) > 15 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// validNumberList splits a comma list, trims each item and keeps the valid
// ones.
func validNumberList(raw string) []string {
	var out []string
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if isEmergencyNumber(item) {
			out = append(out, item)
		}
	}
	return out
}

// LoadEmergencyNumbers reads the three settings through lookup (os.LookupEnv
// in main.go). A variable that is unset keeps its default; a variable that is
// set but empty clears its list; invalid items are dropped; an invalid or
// empty primary falls back to 112.
func LoadEmergencyNumbers(lookup func(string) (string, bool)) EmergencyNumbers {
	n := DefaultEmergencyNumbers()

	if raw, ok := lookup(EmergencyPrimaryEnv); ok {
		raw = strings.TrimSpace(raw)
		if isEmergencyNumber(raw) {
			n.Primary = raw
		}
	}

	if raw, ok := lookup(EmergencyOlderEnv); ok {
		n.Older = validNumberList(raw)
	}

	if raw, ok := lookup(EmergencyStateLinesEnv); ok {
		lines := validNumberList(raw)
		n.StateHelpline = ""
		n.DistrictHelpline = ""
		if len(lines) > 0 {
			n.StateHelpline = lines[0]
		}
		if len(lines) > 1 {
			n.DistrictHelpline = lines[1]
		}
	}

	return n
}

// Chrome is the data every full page shares for the laptop notice strip and
// the legal pages. It is one value built by main.go from env and passed
// through api.Deps.Chrome. LowDataPage is set only by the Low data page
// (04-18). The field that marked the Disclaimer page as current went with the
// bar it served (Phase 3.4, 03.4-05).
type Chrome struct {
	Numbers      EmergencyNumbers
	ContactEmail string
	LowDataPage  bool
}

// withDefaults returns a copy whose Numbers is the default set when the
// primary number is empty, so a zero Chrome (any Deps literal or direct handler
// call in a test) still renders the exact default sentence. It mirrors the
// RequestLinkRateLimit zero value default in router.go. LoadEmergencyNumbers
// always sets Primary, so a cleared older list is never refilled here.
func (c Chrome) withDefaults() Chrome {
	if c.Numbers.Primary == "" {
		c.Numbers = DefaultEmergencyNumbers()
	}
	return c
}
