package service

// KeptByReliableFilter reports whether a report stays when the reader has
// turned on the Show menu's Only Reliable reporters tick (NOTIF-04, D-26). It
// is a server authored field (TRUST-02): the browser reads it and never
// recomputes it, and no tag rule or threshold is sent to the browser.
//
// A report is kept when its reporter's tag is Reliable reporter, or when the
// report is serious. Serious is ReportMeta.Serious, the one critical rule
// (R-04): a critical severity, or a rescue_needed category on a row stored
// before that category was retired. There is no second critical rule here, so
// old rescue_needed rows stay kept exactly as they stay shown by the critical
// bypass.
//
// The tag is the one the report's own Trust object carries, so the tick and the
// reporter line on the card can never disagree. Resolve never reads this
// function, and nothing hides, reorders or changes a report by default: the tick
// is the reader's own opt in choice and is not remembered, so Phase 3 D-14
// (the tag is display only) holds for every default view (ROADMAP 3.2 scope
// note 5).
//
// Known limit: past MaxReporterAccountsPerRead reporter accounts in one read the
// extra reporters carry the none tag, so in a very dense read a far, non
// serious report by a Reliable reporter may be left out of the tick (research
// Pitfall 8). It errs toward showing fewer, never toward showing a report as
// reliable that is not.
func KeptByReliableFilter(tag ReporterTag, meta ReportMeta) bool {
	return tag == ReporterReliable || meta.Serious()
}
