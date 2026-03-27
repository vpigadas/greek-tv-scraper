package xmltv

import (
	"compress/gzip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vpigadas/greek-tv-scraper/internal/model"
)

// XMLTVTime parses XMLTV timestamp format: "20260326200000 +0200"
type XMLTVTime struct{ time.Time }

func (t *XMLTVTime) UnmarshalXMLAttr(attr xml.Attr) error {
	parsed, err := time.Parse("20060102150405 -0700", attr.Value)
	if err != nil {
		parsed, err = time.Parse("20060102150405", attr.Value)
		if err != nil {
			return fmt.Errorf("xmltv time parse error %q: %w", attr.Value, err)
		}
	}
	t.Time = parsed.UTC()
	return nil
}

type xmltvChannel struct {
	ID          string `xml:"id,attr"`
	DisplayName string `xml:"display-name"`
	Icon        struct {
		Src string `xml:"src,attr"`
	} `xml:"icon"`
}

type xmltvProg struct {
	Channel  string    `xml:"channel,attr"`
	Start    XMLTVTime `xml:"start,attr"`
	Stop     XMLTVTime `xml:"stop,attr"`
	Title    string    `xml:"title"`
	Desc     string    `xml:"desc"`
	Category string    `xml:"category"`
	Icon     struct {
		Src string `xml:"src,attr"`
	} `xml:"icon"`
}

// Fetch downloads and parses the XMLTV .gz feed.
var httpClient = &http.Client{
	Timeout: 60 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:    10,
		IdleConnTimeout: 90 * time.Second,
	},
}

// Returns a map of channelID -> []Programme for all channels in the feed.
func Fetch(ctx context.Context, feedURL string, athens *time.Location) (map[string][]model.Programme, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("xmltv: build request: %w", err)
	}
	req.Header.Set("User-Agent", "greek-tv-scraper/1.0")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("xmltv: fetch %s: %w", feedURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("xmltv: unexpected status %d", resp.StatusCode)
	}

	var reader io.Reader = resp.Body
	if strings.HasSuffix(feedURL, ".gz") || resp.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("xmltv: gzip reader: %w", err)
		}
		defer gz.Close()
		reader = gz
	}

	result := make(map[string][]model.Programme)
	decoder := xml.NewDecoder(reader)

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("xmltv: xml decode: %w", err)
		}

		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}

		if start.Name.Local == "programme" {
			var raw xmltvProg
			if err := decoder.DecodeElement(&raw, &start); err != nil {
				continue
			}
			if raw.Channel == "" || raw.Start.IsZero() {
				continue
			}

			athensStart := raw.Start.In(athens)
			athensEnd := raw.Stop.In(athens)
			duration := int(raw.Stop.Sub(raw.Start.Time).Minutes())

			prog := model.Programme{
				ChannelID:   raw.Channel,
				Title:       raw.Title,
				Description: raw.Desc,
				StartTime:   raw.Start.UTC(),
				EndTime:     raw.Stop.UTC(),
				StartLocal:  athensStart.Format("15:04"),
				EndLocal:    athensEnd.Format("15:04"),
				DateLocal:   athensStart.Format("2006-01-02"),
				CoverURL:    raw.Icon.Src,
				Category:    raw.Category,
				Duration:    duration,
			}
			result[raw.Channel] = append(result[raw.Channel], prog)
		}
	}

	return result, nil
}
