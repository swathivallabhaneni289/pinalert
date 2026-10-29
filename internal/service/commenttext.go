package service

// commenttext.go is the text rule of a comment (Phase 8 plan 09, PAGE-02,
// research "Text rules"). The server is the authority for the 280 rule and for
// the character rule; the browser counts the same code points (Array.from) so
// its counter agrees. Invisible and direction characters are refused so nobody
// can hide a spoofed official name behind a zero width space or reverse the
// reading order of a line with an override or an isolate. The two joiners, the
// zero width non joiner U+200C and the zero width joiner U+200D, are the one
// exception: Indic scripts typed on common keyboards and joined emoji need
// them, so they pass, but only where they follow a letter, a mark or a symbol,
// and never as the only content.
//
// CleanCommentBody is only the shape of the text. Whether the words are
// acceptable is the abuse screen's job, ScreenAbuse, which the comment service
// calls. The gibberish prefilter is never used for a comment, because
// "Thanks", "ok" and a lone emoji are real replies (research Pitfall 5).

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// CommentMaxRunes is the longest comment, counted in Unicode code points the
// way a flag note and a report description are (research Pitfall 7).
const CommentMaxRunes = 280

var (
	// ErrCommentEmpty is returned for text with nothing to post, including text
	// made only of spaces and joiners.
	ErrCommentEmpty = errors.New("service: comment is empty")
	// ErrCommentTooLong is returned for text over CommentMaxRunes code points.
	ErrCommentTooLong = errors.New("service: comment is too long")
	// ErrCommentCharacters is returned for text holding a control character, an
	// invisible or direction format character, or a misplaced joiner.
	ErrCommentCharacters = errors.New("service: comment has characters that cannot be posted")
)

// isCommentJoiner reports the two joiners that Indic text and joined emoji
// need. The constants are declared in profile.go.
func isCommentJoiner(r rune) bool {
	return r == zeroWidthNonJoiner || r == zeroWidthJoiner
}

// CleanCommentBody returns the text of a comment ready to store, or one of
// ErrCommentEmpty, ErrCommentCharacters and ErrCommentTooLong. The order is
// part of the specification:
//
//  1. Carriage return line feed becomes a line feed, a tab becomes a space
//     (text pasted from a notes app or WhatsApp holds tabs, review IN-S01), and
//     the ends are trimmed.
//  2. Any control character other than a line feed, and any format character
//     (unicode.Cf) other than the two joiners, answers ErrCommentCharacters.
//  3. Joiners and white space are trimmed from both ends together until
//     nothing changes, and a result with no letter, mark, number, punctuation
//     or symbol answers ErrCommentEmpty, so a body of joiners never posts.
//  4. Inside the trimmed text every joiner must follow a letter, a mark or a
//     symbol, else ErrCommentCharacters.
//  5. Runs of three or more line feeds collapse to two.
//  6. More than CommentMaxRunes code points answers ErrCommentTooLong.
func CleanCommentBody(s string) (string, error) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.TrimSpace(s)

	for _, r := range s {
		if r == '\n' {
			continue
		}
		if unicode.IsControl(r) {
			return "", ErrCommentCharacters
		}
		if unicode.Is(unicode.Cf, r) && !isCommentJoiner(r) {
			return "", ErrCommentCharacters
		}
	}

	// One TrimFunc removes joiners and white space in any interleaving, so
	// there is nothing to repeat.
	s = strings.TrimFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || isCommentJoiner(r)
	})
	hasContent := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) ||
			unicode.IsPunct(r) || unicode.IsSymbol(r) {
			hasContent = true
			break
		}
	}
	if !hasContent {
		return "", ErrCommentEmpty
	}

	var prev rune
	for _, r := range s {
		if isCommentJoiner(r) {
			if !(unicode.IsLetter(prev) || unicode.IsMark(prev) || unicode.IsSymbol(prev)) {
				return "", ErrCommentCharacters
			}
		}
		prev = r
	}

	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}

	if utf8.RuneCountInString(s) > CommentMaxRunes {
		return "", ErrCommentTooLong
	}
	return s, nil
}
