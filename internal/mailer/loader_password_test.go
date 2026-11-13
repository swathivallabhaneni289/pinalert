package mailer

import "testing"

// TestLoadMailerAppPasswordSpaces pins IN-06: the spaces Google shows in an app
// password are dropped only for smtp.gmail.com and only when what is left is 16
// letters. Any other value, or any other host, is used exactly as given.
func TestLoadMailerAppPasswordSpaces(t *testing.T) {
	for _, tc := range []struct {
		name     string
		host     string
		password string
		want     string
	}{
		{"gmail, four groups of four letters", "smtp.gmail.com", "abcd efgh ijkl mnop", "abcdefghijklmnop"},
		{"gmail, capitals and uneven groups", "smtp.gmail.com", "ABcd efghijkl mn op", "ABcdefghijklmnop"},
		{"gmail, host in capitals", "SMTP.Gmail.com", "abcd efgh ijkl mnop", "abcdefghijklmnop"},
		{"gmail, already without spaces", "smtp.gmail.com", "abcdefghijklmnop", "abcdefghijklmnop"},
		{"gmail, 15 letters keeps its spaces", "smtp.gmail.com", "abcd efgh ijkl mno", "abcd efgh ijkl mno"},
		{"gmail, 17 letters keeps its spaces", "smtp.gmail.com", "abcd efgh ijkl mnopq", "abcd efgh ijkl mnopq"},
		{"gmail, a digit keeps its spaces", "smtp.gmail.com", "abcd efgh ijkl mno1", "abcd efgh ijkl mno1"},
		{"gmail, a symbol keeps its spaces", "smtp.gmail.com", "abcd efgh ijkl mno-", "abcd efgh ijkl mno-"},
		{"other host, 16 letters in groups keeps its spaces", "smtp.example.com", "abcd efgh ijkl mnop", "abcd efgh ijkl mnop"},
		{"other host, a passphrase keeps its spaces", "smtp.example.com", "correct horse battery", "correct horse battery"},
		{"a host that only starts like gmail keeps its spaces", "smtp.gmail.com.example.net", "abcd efgh ijkl mnop", "abcd efgh ijkl mnop"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setSMTPEnv(t, tc.host, "", testSMTPUser, tc.password, "")
			m, err := LoadMailer("production")
			if err != nil {
				t.Fatalf("LoadMailer: %v", err)
			}
			if got := m.(*smtpMailer).cfg.Password; got != tc.want {
				t.Errorf("password = %q, want %q", got, tc.want)
			}
		})
	}
}
