package service

// people_cursor_seal_test.go pins SK-W03 of the Phase 9 skeptic round (2026-10-30):
// the people list cursor is sealed, not only signed, so the follow time never
// leaves the server in clear, and it is bound to the list it was issued for.
// No database.

import (
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The follow time and the username must not be readable from a cursor by
// decoding any part of it.
func TestPeopleCursorHidesTheFollowTime(t *testing.T) {
	at := time.Date(2026, 10, 1, 8, 30, 0, 123000, time.UTC)
	micros := strconv.FormatInt(at.UnixMicro(), 10)
	cursor := encodePeopleCursor(7, true, at, "asha_k")
	for _, part := range strings.FieldsFunc(cursor, func(r rune) bool { return r == '.' }) {
		if raw, err := base64.RawURLEncoding.DecodeString(part); err == nil {
			if strings.Contains(string(raw), micros) || strings.Contains(string(raw), "asha_k") {
				t.Fatalf("part %q of the cursor decodes to a readable time or name: %q", part, raw)
			}
		}
	}
	if strings.Contains(cursor, micros) {
		t.Fatalf("the cursor %q holds the time in clear", cursor)
	}
}

// Two cursors for the same row differ (a random nonce), so a repeated page
// cannot be recognised from its cursor.
func TestPeopleCursorUsesARandomNonce(t *testing.T) {
	at := time.Date(2026, 10, 1, 8, 30, 0, 123000, time.UTC)
	a := encodePeopleCursor(7, true, at, "asha_k")
	b := encodePeopleCursor(7, true, at, "asha_k")
	if a == b {
		t.Fatal("two cursors for the same row are identical")
	}
}

// A cursor opens only on the list it was issued for: the same owner and the
// same direction. Any other list gets the same calm refusal.
func TestPeopleCursorIsBoundToItsList(t *testing.T) {
	at := time.Date(2026, 10, 1, 8, 30, 0, 123000, time.UTC)
	issued := encodePeopleCursor(7, true, at, "asha_k")

	if _, _, err := decodePeopleCursor(issued, 7, true); err != nil {
		t.Fatalf("the list it was issued for refused it: %v", err)
	}
	for name, c := range map[string]struct {
		owner     int64
		followers bool
	}{
		"another owner, same direction": {8, true},
		"same owner, the other list":    {7, false},
		"another owner, other list":     {8, false},
	} {
		_, _, err := decodePeopleCursor(issued, c.owner, c.followers)
		ve, ok := err.(ValidationError)
		if !ok || ve.Field != "after" || ve.Message != peopleCursorMessage {
			t.Errorf("%s: err = %#v, want the calm after refusal", name, err)
		}
	}
}
