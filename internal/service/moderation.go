// moderation.go holds the moderator side of D-08: the three lists, Find, Keep,
// Remove and the server list of moderator emails. Moderators and the author are
// the only people who see a held or removed report (R-01).

package service

import (
	"fmt"
	"strings"
)

// ParseModeratorEmails turns the MODERATOR_EMAILS setting into the set of
// addresses allowed to open the moderator screen (D-08, T-04-07).
//
// Steps:
//  1. Split raw on commas.
//  2. Trim each item and skip it when empty.
//  3. Normalise it with NormalizeEmail, the rule sign in uses, so a mixed case
//     entry still matches the stored address of the account.
//  4. Collect into a set, which drops duplicates.
//
// A bad entry returns a nil map and an error that names the entry by its 1
// based position among the items as written, empty ones included, and never by
// its value, so a mistyped address cannot reach a log (T-04-07). An empty or
// all blank setting returns an empty, non nil set and no error: nobody is a
// moderator.
func ParseModeratorEmails(raw string) (map[string]struct{}, error) {
	set := make(map[string]struct{})
	for i, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		email, err := NormalizeEmail(item)
		if err != nil {
			return nil, fmt.Errorf("entry %d is not a bare email address", i+1)
		}
		set[email] = struct{}{}
	}
	return set, nil
}
