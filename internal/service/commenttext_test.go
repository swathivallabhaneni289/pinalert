package service_test

// commenttext_test.go proves the text rule of a comment (Phase 8 plan 09,
// PAGE-02, SC3, review W1): the cleaning, the 280 code point bound, the refusal
// of control and invisible characters, the two joiners that Indic text and
// joined emoji need, and the short replies that the gibberish prefilter would
// have refused. Characters that are invisible or easy to mangle are written as
// rune values, never as raw characters.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"pinalert/internal/moderation"
	"pinalert/internal/service"
)

// cmtRunes builds a string from code points.
func cmtRunes(rs ...rune) string { return string(rs) }

// cmtZWSP and friends are the characters the rule is about.
var (
	cmtZWSP = cmtRunes(0x200B)
	cmtZWNJ = cmtRunes(0x200C)
	cmtZWJ  = cmtRunes(0x200D)
	cmtBOM  = cmtRunes(0xFEFF)
)

func TestCleanCommentBody(t *testing.T) {
	emoji := cmtRunes(0x1F600) // a 4 byte code point
	teluguThanks := cmtRunes(0x0C27, 0x0C28, 0x0C4D, 0x0C2F, 0x0C35, 0x0C3E, 0x0C26, 0x0C3E, 0x0C32, 0x0C41)
	// The family emoji: man, joiner, woman, joiner, girl, joiner, boy.
	family := cmtRunes(0x1F468, 0x200D, 0x1F469, 0x200D, 0x1F467, 0x200D, 0x1F466)
	// A thumbs up with a skin tone, then a joiner, then a heart with the
	// variation selector 16: tone is a modifier symbol, selector a mark.
	toneThenJoined := cmtRunes(0x1F44D, 0x1F3FD, 0x200D, 0x2764, 0xFE0F)
	// A Malayalam word with the zero width non joiner after a chillu base.
	malayalam := cmtRunes(0x0D28, 0x0D4D, 0x200C, 0x0D2E)
	// A Hindi word whose virama is followed by a zero width joiner.
	hindi := cmtRunes(0x0915, 0x094D, 0x200D, 0x0937)

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{"trims the ends", "  Thanks  ", "Thanks", nil},
		{"empty", "", "", service.ErrCommentEmpty},
		{"only spaces", "   \n  ", "", service.ErrCommentEmpty},
		{"a lone joiner", cmtZWJ, "", service.ErrCommentEmpty},
		{"a run of both joiners", cmtZWJ + cmtZWNJ + cmtZWJ, "", service.ErrCommentEmpty},
		{"a joiner padded with spaces", " " + cmtZWJ + " ", "", service.ErrCommentEmpty},
		{"joiners and spaces interleaved", cmtZWJ + " " + cmtZWNJ + " " + cmtZWJ, "", service.ErrCommentEmpty},
		{"joiner at each end of ok", cmtZWJ + "ok" + cmtZWNJ, "ok", nil},
		{"280 four byte emoji pass", strings.Repeat(emoji, 280), strings.Repeat(emoji, 280), nil},
		{"281 emoji are too long", strings.Repeat(emoji, 281), "", service.ErrCommentTooLong},
		{"a Telugu word counts code points", strings.Repeat(teluguThanks, 28), strings.Repeat(teluguThanks, 28), nil},
		{"281 Telugu code points are too long", strings.Repeat(teluguThanks, 28) + cmtRunes(0x0C27), "", service.ErrCommentTooLong},
		{"zero width space", "a" + cmtZWSP + "b", "", service.ErrCommentCharacters},
		{"zero width space alone", cmtZWSP, "", service.ErrCommentCharacters},
		{"byte order mark", cmtBOM + "hello", "", service.ErrCommentCharacters},
		{"left to right mark", "a" + cmtRunes(0x200E) + "b", "", service.ErrCommentCharacters},
		{"right to left mark", "a" + cmtRunes(0x200F) + "b", "", service.ErrCommentCharacters},
		{"override 202A", "a" + cmtRunes(0x202A) + "b", "", service.ErrCommentCharacters},
		{"override 202B", "a" + cmtRunes(0x202B) + "b", "", service.ErrCommentCharacters},
		{"override 202C", "a" + cmtRunes(0x202C) + "b", "", service.ErrCommentCharacters},
		{"override 202D", "a" + cmtRunes(0x202D) + "b", "", service.ErrCommentCharacters},
		{"override 202E", "a" + cmtRunes(0x202E) + "b", "", service.ErrCommentCharacters},
		{"isolate 2066", "a" + cmtRunes(0x2066) + "b", "", service.ErrCommentCharacters},
		{"isolate 2067", "a" + cmtRunes(0x2067) + "b", "", service.ErrCommentCharacters},
		{"isolate 2068", "a" + cmtRunes(0x2068) + "b", "", service.ErrCommentCharacters},
		{"isolate 2069", "a" + cmtRunes(0x2069) + "b", "", service.ErrCommentCharacters},
		// Amended 2026-10-29 by review IN-S01: a tab pasted from a notes app or
		// WhatsApp was refused; it is now read as a space, and an end tab trims.
		{"a tab", "a\tb", "a b", nil},
		{"tabs at the ends", "\ta b\t", "a b", nil},
		{"only tabs", "\t\t", "", service.ErrCommentEmpty},
		{"the bell character", "a" + cmtRunes(7) + "b", "", service.ErrCommentCharacters},
		{"a lone carriage return", "a\rb", "", service.ErrCommentCharacters},
		{"a Malayalam word with the non joiner", malayalam, malayalam, nil},
		{"a Hindi word with the joiner", hindi, hindi, nil},
		{"a joined emoji family", family, family, nil},
		{"a skin tone and a variation selector before joiners", toneThenJoined, toneThenJoined, nil},
		{"a joiner after a space inside the text", "a " + cmtZWJ + "b", "", service.ErrCommentCharacters},
		{"two joiners in a row inside the text", "a" + cmtZWJ + cmtZWJ + "b", "", service.ErrCommentCharacters},
		{"a joiner after a digit", "5" + cmtZWNJ + "6", "", service.ErrCommentCharacters},
		{"carriage return line feed becomes a line feed", "a\r\nb", "a\nb", nil},
		{"three line feeds collapse to two", "a\n\n\n\nb", "a\n\nb", nil},
		{"two line feeds stay", "a\n\nb", "a\n\nb", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := service.CleanCommentBody(tc.in)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CleanCommentBody error = %v, want %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("CleanCommentBody = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCommentShortReplies shows that the replies people really send pass the
// text rule and the abuse screen, and that they are exactly the texts the
// gibberish prefilter would have refused, which is why the comment service
// calls ScreenAbuse and never CheckText (research Pitfall 5).
func TestCommentShortReplies(t *testing.T) {
	replies := map[string]string{
		"English thanks": "Thanks",
		"English ok":     "ok",
		"a lone emoji":   cmtRunes(0x1F44D),
		"Hindi thanks":   cmtRunes(0x0927, 0x0928, 0x094D, 0x092F, 0x0935, 0x093E, 0x0926),
		"Telugu thanks":  cmtRunes(0x0C27, 0x0C28, 0x0C4D, 0x0C2F, 0x0C35, 0x0C3E, 0x0C26, 0x0C3E, 0x0C32, 0x0C41),
	}
	for name, text := range replies {
		t.Run(name, func(t *testing.T) {
			cleaned, err := service.CleanCommentBody(text)
			if err != nil || cleaned != text {
				t.Fatalf("CleanCommentBody(%q) = %q, %v, want it unchanged", text, cleaned, err)
			}
			if !service.IsGibberish(cleaned) {
				t.Fatalf("IsGibberish(%q) = false: this row no longer shows why CheckText is the wrong call", cleaned)
			}
			scr := &fakeTextScreener{result: moderation.Result{Scores: map[string]float64{"hate": 0.01}}}
			check := service.ScreenAbuse(context.Background(), scr, cleaned)
			if check.Outcome != service.TextClear {
				t.Errorf("ScreenAbuse(%q) = %q, want clear", cleaned, check.Outcome)
			}
			if scr.callCount() != 1 {
				t.Errorf("screener calls = %d, want 1", scr.callCount())
			}
		})
	}
}

func TestCommentCopyWording(t *testing.T) {
	messages := map[string]string{
		"CommentAuthorNewMember":      service.CommentAuthorNewMember,
		"CommentAuthorDeletedAccount": service.CommentAuthorDeletedAccount,
		"CommentsClosedMessage":       service.CommentsClosedMessage,
		"CommentEmptyMessage":         service.CommentEmptyMessage,
		"CommentTooLongMessage":       service.CommentTooLongMessage,
		"CommentCharactersMessage":    service.CommentCharactersMessage,
		"CommentNeedsUsernameMessage": service.CommentNeedsUsernameMessage,
		"CommentGoneMessage":          service.CommentGoneMessage,
		"CommentNotYoursMessage":      service.CommentNotYoursMessage,
		"CommentOwnFlagMessage":       service.CommentOwnFlagMessage,
	}
	for name, msg := range messages {
		t.Run(name, func(t *testing.T) {
			assertServerCopy(t, msg)
			lower := strings.ToLower(msg)
			for _, banned := range []string{"sound", "buzz", "vibrat", "ring", "beep"} {
				if strings.Contains(lower, banned) {
					t.Errorf("%s = %q names a sound (%q)", name, msg, banned)
				}
			}
		})
	}
	// No username can equal the two author words: usernames are lowercase and
	// have no spaces.
	for _, w := range []string{service.CommentAuthorNewMember, service.CommentAuthorDeletedAccount} {
		if !strings.Contains(w, " ") || strings.ToLower(w) == w {
			t.Errorf("%q must hold a space and a capital", w)
		}
	}
	if service.CommentsClosedMessage != "Comments are closed" {
		t.Errorf("CommentsClosedMessage = %q", service.CommentsClosedMessage)
	}
}
