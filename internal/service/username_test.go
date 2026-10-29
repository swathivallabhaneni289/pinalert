package service

import (
	"strings"
	"testing"
)

func TestUsernameNormalize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  Ravi.K ", "ravi.k"},
		{"@ravi_k", "ravi_k"},
		{"@@ravi", "@ravi"},
		{"RAVI", "ravi"},
		{"", ""},
		{"  @Asha_1\t", "asha_1"},
		// Case folding is ASCII only: a non ASCII capital stays as typed.
		{"R" + string(rune(0xc1)) + "VI", "r" + string(rune(0xc1)) + "vi"},
	}
	for _, c := range cases {
		if got := NormalizeUsername(c.in); got != c.want {
			t.Errorf("NormalizeUsername(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestUsernameClassify(t *testing.T) {
	twenty := strings.Repeat("a", 20)
	twentyOne := strings.Repeat("a", 21)
	cases := []struct {
		name string
		in   string
		want UsernameClass
	}{
		{"dotted", "ravi.k", UsernameUsable},
		{"underscore and digits", "asha_123", UsernameUsable},
		{"three characters", "a1b", UsernameUsable},
		{"both separators", "x_y.z", UsernameUsable},
		{"twenty characters", twenty, UsernameUsable},
		{"capitals and spaces are normalised first", "  Ravi.K ", UsernameUsable},
		{"leading at is dropped", "@ravi_k", UsernameUsable},

		{"empty", "", UsernameInvalid},
		{"two characters", "ab", UsernameInvalid},
		{"twenty one characters", twentyOne, UsernameInvalid},
		{"no letter", "123", UsernameInvalid},
		{"no letter with separators", "1_2.3", UsernameInvalid},
		{"leading full stop", ".ravi", UsernameInvalid},
		{"trailing full stop", "ravi.", UsernameInvalid},
		{"double full stop", "ra..vi", UsernameInvalid},
		{"space inside", "ravi k", UsernameInvalid},
		{"hyphen", "ravi-k", UsernameInvalid},
		{"non ASCII letter", "r" + string(rune(0xe1)) + "vi", UsernameInvalid},
		{"zero width space U+200B", "ravi" + string(rune(0x200b)), UsernameInvalid},
		{"devanagari", "ravi" + string(rune(0x0915)), UsernameInvalid},
		{"two at signs", "@@ravi", UsernameInvalid},
		{"markup", "<b>ravi</b>", UsernameInvalid},

		{"police", "police", UsernameIsReserved},
		{"police suffix", "police_hyd", UsernameIsReserved},
		{"police token", "hyd.police", UsernameIsReserved},
		{"capital reserved", "Police", UsernameIsReserved},
		{"admin look alike", "adm1n", UsernameIsReserved},
		{"imd", "imd", UsernameIsReserved},

		{"imdone is a different word", "imdone", UsernameUsable},
		{"police with a zero", "p0lice", UsernameIsReserved},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			name, got := ClassifyUsername(c.in)
			if got != c.want {
				t.Fatalf("ClassifyUsername(%q) class = %d, want %d", c.in, got, c.want)
			}
			if want := NormalizeUsername(c.in); name != want {
				t.Fatalf("ClassifyUsername(%q) name = %q, want the normalised %q", c.in, name, want)
			}
		})
	}
}

func TestReservedUsernames(t *testing.T) {
	reserved := []string{
		"police", "police_hyd", "hyd.police", "p0lice", "ndma.india", "ndrf", "govt",
		"government", "official", "pinalert", "pinalert_team", "moderator",
		"administrator", "emergency", "fire", "ambulance", "imd", "gov", "mod",
		"admin", "adm1n", "help", "support", "1md", "sdma_ap", "100", "112",
		"sdrf", "firefly", "po1ice", "po11ce", "p01ice", "ambu1ance", "3mergency",
		"adm.in", "the_admin", "ad_min",
	}
	for _, n := range reserved {
		if !UsernameReserved(n) {
			t.Errorf("UsernameReserved(%q) = false, want true", n)
		}
	}
	free := []string{
		"imdone", "modest", "helpful", "supporter", "governor", "admiral", "asha",
		"ravi.k", "asha_123", "mohan", "gover", "firm", "officer", "ndm", "pina",
	}
	for _, n := range free {
		if UsernameReserved(n) {
			t.Errorf("UsernameReserved(%q) = true, want false", n)
		}
	}
	// A long run of ones must stay cheap and must not hang or blow up.
	if UsernameReserved(strings.Repeat("1", 18) + "x") {
		t.Errorf("a run of ones with x should not be reserved")
	}
}

func TestUsernameScreenText(t *testing.T) {
	cases := []struct{ in, want string }{
		{"ravi_k.123", "ravi k 123"},
		{"ravi", "ravi"},
		{"a.b_c", "a b c"},
	}
	for _, c := range cases {
		if got := UsernameScreenText(c.in); got != c.want {
			t.Errorf("UsernameScreenText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestUsernameMessages(t *testing.T) {
	if UsernameMinLen != 3 || UsernameMaxLen != 20 {
		t.Errorf("length bounds = %d to %d, want 3 to 20", UsernameMinLen, UsernameMaxLen)
	}
	if UsernameRenameGap.Hours() != 7*24 {
		t.Errorf("UsernameRenameGap = %v, want 7 days", UsernameRenameGap)
	}
	exact := []struct{ got, want string }{
		{UsernameRuleMessage, "Use 3 to 20 letters, numbers, dots or underscores."},
		{UsernameReservedMessage, "That name is kept for official use."},
		{UsernameRefusedMessage, "That username can't be used. Try another."},
		{UsernameYoursMessage, "This is your username now."},
		{UsernameCheckFailedMessage, "Couldn't check that username. Try again."},
		{UsernameTakenMessage("ravi.k"), "ravi.k is taken. Try another."},
		{UsernameAvailableMessage("ravi.k"), "ravi.k is available."},
		{UsernameWaitMessage(1), "You can change your username again in 1 day."},
		{UsernameWaitMessage(3), "You can change your username again in 3 days."},
		{UsernameWaitMessage(7), "You can change your username again in 7 days."},
	}
	for _, c := range exact {
		if c.got != c.want {
			t.Errorf("message = %q, want %q", c.got, c.want)
		}
		if strings.ContainsRune(c.got, rune(0x2014)) || strings.ContainsRune(c.got, rune(0x2013)) {
			t.Errorf("message %q contains a dash character", c.got)
		}
	}
}
