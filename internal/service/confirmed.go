package service

// ConfirmedByPeople reports whether a report belongs in the Show menu's
// Confirmed choice: people nearby have confirmed it. It is a server authored
// field (TRUST-02): the browser renders it and never recomputes it, and no
// threshold is sent to the browser.
//
// Confirmed means confirmed by people. The critical bypass in Resolve decides
// that a Critical report is shown at once; it does not make the report
// confirmed. So the rule is:
//
//   - The report is active: v is Live and the partition word is active, which
//     leaves out Retracted, Withheld, Hidden, Provisional, expired and any
//     visibility this package does not know.
//   - Its distinct confirm cells meet IndependentAgreementThreshold, the one
//     shared independence number.
//   - Its disputes are not a majority at the threshold (disputeMajority). For a
//     non critical report that case is already Hidden and so already out. For a
//     Critical report the bypass keeps it Live however it is disputed, so the
//     majority is checked here: a Critical report people confirmed is in
//     Confirmed unless the disputes outweigh the confirmations.
//
// An unconfirmed Critical report (fewer than the threshold of confirm cells) is
// in the All list but not in Confirmed. Old reports stored as rescue_needed keep
// the bypass and follow the same rule.
//
// This is the default pending the owner's answer on whether a confirmed
// Critical report belongs in Confirmed. If the answer is no, the function takes
// the report's ReportMeta as well and returns false when meta.Serious(), and the
// Critical rows of TestConfirmedByPeople flip with it. ConfirmedByPeople is never
// read by Resolve, ViewOf or BuildTrust.
func ConfirmedByPeople(v Visibility, view FeedView, tally VoteTally) bool {
	if view != FeedViewActive || v != VisibilityLive {
		return false
	}
	if tally.ConfirmCells < IndependentAgreementThreshold {
		return false
	}
	return !disputeMajority(tally)
}

// disputeMajority is the expression of Resolve's Hidden rung (rung 5): a floor
// of IndependentAgreementThreshold dispute cells and a strict majority over the
// confirm cells. It is written out here because the critical bypass sits above
// that rung in Resolve, so Resolve never evaluates it for a Critical report.
// It mirrors Resolve and must change with it;
// TestConfirmedByPeopleAgreesWithResolveForNonCritical pins the two together.
func disputeMajority(t VoteTally) bool {
	return t.DisputeCells >= IndependentAgreementThreshold && t.DisputeCells > t.ConfirmCells
}
