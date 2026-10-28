package areas

import (
	"strings"
	"testing"
)

func TestStatesCoverFile(t *testing.T) {
	ix := Default()
	states := ix.States()
	if len(states) < 30 {
		t.Fatalf("only %d states in the file", len(states))
	}
	for _, s := range states {
		byName, ok := ix.StateByName(s.Name)
		if !ok || byName.Code != s.Code {
			t.Errorf("StateByName(%q) = %+v, %v", s.Name, byName, ok)
		}
		sender := strings.ReplaceAll(s.Name, " ", "-") + "-SDMA"
		bySender, ok := ix.StateForSender(sender)
		if !ok || bySender.Code != s.Code {
			t.Errorf("StateForSender(%q) = %+v, %v", sender, bySender, ok)
		}
	}
}

func TestStateForSender(t *testing.T) {
	ix := Default()
	yes := map[string]string{
		"Andhra-Pradesh-SDMA":    "andhra pradesh",
		"Maharashtra-SDMA":       "maharashtra",
		"andhra-pradesh-sdma":    "andhra pradesh",
		"Tamil-Nadu-SDMA":        "tamil nadu",
		"Jammu-and-Kashmir-SDMA": "jammu & kashmir",
		"Orissa-SDMA":            "odisha",
	}
	for sender, want := range yes {
		s, ok := ix.StateForSender(sender)
		if !ok {
			t.Errorf("%q: not found", sender)
			continue
		}
		if strings.ToLower(s.Name) != want {
			t.Errorf("%q: got %q want %q", sender, s.Name, want)
		}
	}
	for _, no := range []string{"IMD-Chennai", "CWC", "", "-SDMA", "SDMA", "Atlantis-SDMA"} {
		if s, ok := ix.StateForSender(no); ok {
			t.Errorf("%q should not resolve, got %+v", no, s)
		}
	}
}

func TestTrailingState(t *testing.T) {
	ix := Default()
	yes := map[string]string{
		"Gandak, Dumariaghat, Gopalganj, Bihar":                      "bihar",
		"Chengalpattu, Chennai, Kanchipuram districts of Tamil Nadu": "tamil nadu",
		"Bihar":                     "bihar",
		"Gandak, Gopalganj, Bihar.": "bihar",
	}
	for desc, want := range yes {
		s, ok := ix.TrailingState(desc)
		if !ok {
			t.Errorf("%q: not found", desc)
			continue
		}
		if strings.ToLower(s.Name) != want {
			t.Errorf("%q: got %q want %q", desc, s.Name, want)
		}
	}
	for _, no := range []string{"6 Mandals", "", "Gopalganj, Nowhere"} {
		if s, ok := ix.TrailingState(no); ok {
			t.Errorf("%q should not resolve, got %+v", no, s)
		}
	}
}
