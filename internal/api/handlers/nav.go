package handlers

// The main places of the app (Phase 3.4, D-01, D-10, D-12). One ordered list
// is rendered twice by web/templates/app_bar.html.tmpl, once into the phone
// bottom bar and once into the laptop top bar, so both sizes reach the same
// places under the same names. A place added or renamed here appears on both
// sizes; no label is written anywhere else in Go or in a template.

// The keys of the places a page can mark as the current one. NavSettings is
// not a main place: it has no entry in mainNavItems, so on the Settings screen
// no item is current.
const (
	NavNearby   = "nearby"
	NavAlerts   = "alerts"
	NavActivity = "activity"
	NavSettings = "settings"
)

// NavItem is one main place.
type NavItem struct {
	// Key is the stable name of the place, one of the Nav constants.
	Key string
	// Label is the words shown in both bars.
	Label string
	// Href is the real, linkable address of the place. Every place works
	// without script.
	Href string
	// Icon is the name of the glyph under /static/icons without .svg, drawn by a mask
	// rule named app-icon--<name> in app-bar.css. It is a constant of this list and never
	// a request value.
	Icon string
	// Current marks the place the page is. At most one item is current.
	Current bool
}

// NavView is what the app bar partial reads.
type NavView struct {
	// Items are the main places in order, with Current set on at most one.
	Items []NavItem
	// Current is the key of the page, which may be NavSettings.
	Current string
	// Title is the words in the phone top bar.
	Title string
	// BackHref is the address of the phone back arrow. Empty means the page
	// draws no back arrow (D-11).
	BackHref string
}

// ShowGear reports whether the phone top bar draws the Settings gear. The gear
// is hidden on the Settings screen itself (UI-SPEC 1b). The receiver is a value
// because templates call methods on a copy of the view model.
func (v NavView) ShowGear() bool {
	return v.Current != NavSettings
}

// mainNavItems is the one ordered list of the main places. It is copied by
// navFor and never changed, so a request cannot alter it.
var mainNavItems = []NavItem{
	{Key: NavNearby, Label: "Nearby", Href: "/", Icon: "map-pin"},
	{Key: NavAlerts, Label: "Official alerts", Href: "/alerts", Icon: "megaphone"},
	{Key: NavActivity, Label: "My activity", Href: "/profile", Icon: "user"},
}

// navOption changes a NavView built by navFor.
type navOption func(*NavView)

// navBack gives the phone top bar a back arrow to href.
func navBack(href string) navOption {
	return func(v *NavView) { v.BackHref = href }
}

// navFor builds the bar data for a page. current is one of the Nav constants;
// the matching main place is marked current, and NavSettings (or any other
// key) marks none. The title is the label of the current main place, or the
// words the page passes through navTitle. T-34-13: every word comes from this
// file's constants, so no request value reaches either bar.
func navFor(current string, opts ...navOption) NavView {
	items := make([]NavItem, len(mainNavItems))
	copy(items, mainNavItems)
	v := NavView{Items: items, Current: current}
	for i := range v.Items {
		if v.Items[i].Key == current {
			v.Items[i].Current = true
			v.Title = v.Items[i].Label
		}
	}
	for _, opt := range opts {
		opt(&v)
	}
	return v
}

// navTitle sets the phone top bar title for a page that is not one of the
// main places, such as Settings.
func navTitle(title string) navOption {
	return func(v *NavView) { v.Title = title }
}
