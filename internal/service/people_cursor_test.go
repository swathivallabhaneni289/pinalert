package service

// people_cursor_test.go pins IN-E02 of the Phase 9 code review (2026-10-30): the
// people list cursor is made by the server, so a reader cannot craft a cursor
// at a time of their own choosing and bisect when a named person followed. No
// database.
//
// Amended 2026-10-30 (skeptic SK-W03): the cursor is now sealed with AES-GCM and
// bound to one list, so the helpers take the list owner and direction, and the
// "changed time" case flips a byte of the sealed value instead of editing a
// readable payload (there is none).

import (
	"encoding/base64"
	"strconv"
	"testing"
	"time"
)

func TestPeopleCursorIsSigned(t *testing.T) {
	at := time.Date(2026, 10, 1, 8, 30, 0, 123000, time.UTC)

	t.Run("a cursor the server issued decodes", func(t *testing.T) {
		got, name, err := decodePeopleCursor(encodePeopleCursor(7, true, at, "asha_k"), 7, true)
		if err != nil || name == nil || *name != "asha_k" || !got.Valid || !got.Time.Equal(at) {
			t.Fatalf("decode = %v %v %v, want the same time and name", got, name, err)
		}
	})
	t.Run("an empty cursor is the first page", func(t *testing.T) {
		got, name, err := decodePeopleCursor("", 7, true)
		if err != nil || name != nil || got.Valid {
			t.Fatalf("decode = %v %v %v, want the first page", got, name, err)
		}
	})
	t.Run("a hand made cursor is refused", func(t *testing.T) {
		raw := strconv.FormatInt(at.UnixMicro(), 10) + ":asha_k"
		plain := base64.RawURLEncoding.EncodeToString([]byte(raw))
		for _, crafted := range []string{
			plain,
			plain + ".",
			plain + "." + base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef")),
			"f:7:" + raw,
			base64.RawURLEncoding.EncodeToString([]byte("f:7:" + raw)),
		} {
			if _, _, err := decodePeopleCursor(crafted, 7, true); err == nil {
				t.Errorf("a crafted cursor %q was accepted", crafted)
			}
		}
	})
	t.Run("a changed cursor is refused", func(t *testing.T) {
		issued := encodePeopleCursor(7, true, at, "asha_k")
		sealed, err := base64.RawURLEncoding.DecodeString(issued)
		if err != nil {
			t.Fatalf("the cursor %q is not base64url: %v", issued, err)
		}
		for i := range sealed {
			forged := append([]byte(nil), sealed...)
			forged[i] ^= 0x01
			if _, _, err := decodePeopleCursor(base64.RawURLEncoding.EncodeToString(forged), 7, true); err == nil {
				t.Fatalf("a cursor with byte %d changed was accepted", i)
			}
		}
	})
}
