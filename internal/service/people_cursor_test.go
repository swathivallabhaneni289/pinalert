package service

// people_cursor_test.go pins IN-E02 of the Phase 9 code review (2026-10-30): the
// people list cursor is signed by the server, so a reader cannot craft a cursor
// at a time of their own choosing and bisect when a named person followed. No
// database.

import (
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPeopleCursorIsSigned(t *testing.T) {
	at := time.Date(2026, 10, 1, 8, 30, 0, 123000, time.UTC)

	t.Run("a cursor the server issued decodes", func(t *testing.T) {
		got, name, err := decodePeopleCursor(encodePeopleCursor(at, "asha_k"))
		if err != nil || name == nil || *name != "asha_k" || !got.Valid || !got.Time.Equal(at) {
			t.Fatalf("decode = %v %v %v, want the same time and name", got, name, err)
		}
	})
	t.Run("an empty cursor is the first page", func(t *testing.T) {
		got, name, err := decodePeopleCursor("")
		if err != nil || name != nil || got.Valid {
			t.Fatalf("decode = %v %v %v, want the first page", got, name, err)
		}
	})
	t.Run("a hand made cursor is refused", func(t *testing.T) {
		raw := strconv.FormatInt(at.UnixMicro(), 10) + ":asha_k"
		for _, crafted := range []string{
			base64.RawURLEncoding.EncodeToString([]byte(raw)),
			base64.RawURLEncoding.EncodeToString([]byte(raw)) + ".",
			base64.RawURLEncoding.EncodeToString([]byte(raw)) + "." + base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef")),
		} {
			if _, _, err := decodePeopleCursor(crafted); err == nil {
				t.Errorf("a crafted cursor %q was accepted", crafted)
			}
		}
	})
	t.Run("a changed time is refused", func(t *testing.T) {
		issued := encodePeopleCursor(at, "asha_k")
		payload, tag, found := strings.Cut(issued, ".")
		if !found {
			t.Fatalf("the cursor %q has no signature", issued)
		}
		raw, _ := base64.RawURLEncoding.DecodeString(payload)
		_, name, _ := strings.Cut(string(raw), ":")
		forged := base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(at.Add(time.Hour).UnixMicro(), 10)+":"+name)) + "." + tag
		if _, _, err := decodePeopleCursor(forged); err == nil {
			t.Error("a cursor with a changed time was accepted")
		}
	})
}
