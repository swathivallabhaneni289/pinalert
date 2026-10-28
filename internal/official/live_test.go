package official

// The live check for plan 05-02. It is the only test in this package that calls
// the real SACHET host, and it is off unless OFFICIAL_LIVE=1, the same way
// GEOCODE_LIVE gates the geocoder's live check. Run it by hand, rarely:
//
//	OFFICIAL_LIVE=1 go test ./internal/official/ -run TestOfficialLive -count=1 -v
//
// It makes two requests: one for the India RSS list and one for the CAP message
// of the newest item. It logs the message type, severity and language labels it
// saw, nothing else.

import (
	"context"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestOfficialLive(t *testing.T) {
	if os.Getenv("OFFICIAL_LIVE") != "1" {
		t.Skip("live check calls the real SACHET host; run with OFFICIAL_LIVE=1")
	}

	c := NewSachetClient(ClientConfig{ContactEmail: os.Getenv("CONTACT_EMAIL")})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	list, err := c.FetchList(ctx, "", "")
	if err != nil {
		t.Fatalf("FetchList failed (class %s)", ErrorClass(err))
	}
	if len(list.Items) == 0 {
		t.Fatal("the list holds no items")
	}
	items := append([]Item(nil), list.Items...)
	sort.SliceStable(items, func(i, j int) bool { return items[i].PubDate.After(items[j].PubDate) })
	newest := items[0]
	t.Logf("list: %d items, validators present: etag=%v last-modified=%v",
		len(list.Items), list.ETag != "", list.LastModified != "")

	res := c.FetchCAPs(ctx, []Item{newest})
	if len(res) != 1 {
		t.Fatalf("FetchCAPs returned %d results", len(res))
	}
	if res[0].Err != nil {
		t.Fatalf("CAP fetch failed (class %s)", ErrorClass(res[0].Err))
	}
	v := res[0].Version
	langs := []string{}
	if v.HeadlineEN != "" {
		langs = append(langs, "en")
	}
	if v.OriginalLang != "" {
		langs = append(langs, v.OriginalLang)
	}
	t.Logf("newest message: msgType=%s severity=%s languages=%s kept=%v expiresInferred=%v",
		v.MsgType, v.Severity, strings.Join(langs, ","), res[0].Kept, v.ExpiresInferred)
}
