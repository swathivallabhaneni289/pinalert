package areas

// Display names (WR-S07 of the Phase 5 review). The boundary file keeps the LGD
// dtname as it is, and some of those read badly on a page or a lock screen:
// "Ntr", a misspelt "Visakhapatanam", or Delhi's bare compass words. Load
// applies this table where it builds each District, so every reader of
// District.Name (the alerts page, the Nearby banner, the districts line, a phone
// notification title and the Demo) gets the same words, and a rebuild of the
// embedded file keeps them.
//
// Each entry names the raw LGD name it replaces, so a rebuilt file that moves a
// code to another district keeps that district's own name instead of taking a
// wrong one (TestDisplayNamesApply then fails). Names only: no code compares a
// district name with feed text, placement uses LGD codes and state names.
var displayNames = map[int]struct{ raw, shown string }{
	749: {"Ntr", "NTR"},
	515: {"Spsr Nellore", "Sri Potti Sriramulu Nellore"},
	504: {"Y.S.R.", "YSR Kadapa"},
	520: {"Visakhapatanam", "Visakhapatnam"},
	608: {"S.A.S Nagar", "SAS Nagar"},
	303: {"24 Paraganas North", "North 24 Parganas"},
	304: {"24 Paraganas South", "South 24 Parganas"},
	77:  {"Central", "Central Delhi"},
	78:  {"East", "East Delhi"},
	80:  {"North", "North Delhi"},
	81:  {"North East", "North East Delhi"},
	82:  {"North West", "North West Delhi"},
	83:  {"South", "South Delhi"},
	84:  {"South West", "South West Delhi"},
	85:  {"West", "West Delhi"},
	670: {"South East", "South East Delhi"},
}

// displayName is the name a district is shown by: the table's word when the code
// and its raw LGD name both match, else the LGD name as it is.
func displayName(code int, raw string) string {
	if n, ok := displayNames[code]; ok && n.raw == raw {
		return n.shown
	}
	return raw
}
