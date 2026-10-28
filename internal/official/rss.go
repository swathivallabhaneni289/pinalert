// Package official reads the government alert feeds Pinalert shows beside
// reader reports. This file and cap.go are pure parsing; sachet.go is the
// bounded HTTP client that feeds them. The package never logs: callers log
// closed error words only (see ErrorClass).
package official

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

// ErrParse marks every failure to read a feed body: bad XML, a wrong root
// element, a missing required field, or a body over its size limit. ErrorClass
// reports any error that wraps it as "parse".
var ErrParse = errors.New("official: could not read the feed")

// guidPattern is the shape of a SACHET item id: 10 to 20 digits and nothing
// else. A CAP address is built only from a guid that matches it (research G2).
var guidPattern = regexp.MustCompile(`^[0-9]{10,20}$`)

// ValidGUID reports whether s may be put into a CAP address. The check is on
// the whole string, so a trailing newline, a path part or a query part fails.
func ValidGUID(s string) bool {
	return guidPattern.MatchString(s)
}

// Item is the RSS pointer to one alert. It carries no severity, expiry, area or
// language: those live in the CAP message the guid points to. Link is kept for
// the record only and is never requested.
type Item struct {
	GUID        string
	Title       string
	Link        string
	IssuerLabel string
	PubDate     time.Time
}

type rssDoc struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	Author  string `xml:"author"`
	GUID    string `xml:"guid"`
	PubDate string `xml:"pubDate"`
}

// ParseRSS reads the SACHET India feed into pointers, in feed order. The
// description, the category and the stale channel date are ignored. An item
// whose guid is not a valid id is kept as read: the caller (FetchCAPs) refuses
// it, so a bad item is counted rather than silently lost.
func ParseRSS(r io.Reader) ([]Item, error) {
	var doc rssDoc
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("%w: rss: %s", ErrParse, shortXMLError(err))
	}
	items := make([]Item, 0, len(doc.Channel.Items))
	for _, ri := range doc.Channel.Items {
		items = append(items, Item{
			GUID:        strings.TrimSpace(ri.GUID),
			Title:       CleanText(ri.Title, HeadlineMaxRunes),
			Link:        CleanText(ri.Link, 300),
			IssuerLabel: IssuerLabel(ri.Author),
			PubDate:     parseRSSDate(ri.PubDate),
		})
	}
	return items, nil
}

// IssuerLabel turns an RSS author such as "controlroom@ndma.gov.in (IMD
// Chennai)" into "IMD Chennai". The address is dropped. With no brackets the
// text is the label only when it holds no address.
func IssuerLabel(author string) string {
	author = strings.TrimSpace(author)
	label := author
	if open := strings.LastIndex(author, "("); open >= 0 {
		if end := strings.LastIndex(author, ")"); end > open {
			label = strings.TrimSpace(author[open+1 : end])
		} else {
			label = ""
		}
	}
	if strings.Contains(label, "@") {
		return ""
	}
	return CleanText(label, IssuerMaxRunes)
}

func parseRSSDate(s string) time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC1123, time.RFC1123Z} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// shortXMLError keeps the decoder's message without any of the body.
func shortXMLError(err error) string {
	if err == io.EOF {
		return "empty body"
	}
	var se *xml.SyntaxError
	if errors.As(err, &se) {
		return fmt.Sprintf("syntax error at line %d", se.Line)
	}
	return "unreadable xml"
}
