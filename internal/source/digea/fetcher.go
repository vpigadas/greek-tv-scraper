package digea

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/vpigadas/greek-tv-scraper/internal/model"
)

// DigeaEvent as returned by POST /el/api/epg/get-events.
type DigeaEvent struct {
	ChannelID    string `json:"channel_id"`
	Title        string `json:"title"`
	LongSynopsis string `json:"long_synopsis"`
	ActualTime   string `json:"actual_time"`   // "2026-03-26 01:00:00"
	EndTime      string `json:"end_time"`      // "2026-03-26 02:45:00"
	UTCOffset    string `json:"utc_offset"`    // "2" or "3"
}

// digeaToEPGID maps Digea numeric channel IDs to XMLTV EPG IDs.
var digeaToEPGID = map[string]string{
	"3100": "alpha",
	"1100": "ant1",
	"3000": "mtv",
	"2100": "skai",
	"2000": "star",
	"3200": "open",
	"1000": "mega",
}

// FetchAllEvents fetches all events for a given date via the Digea API.
// Returns a map of EPG channel ID → []Programme.
func FetchAllEvents(ctx context.Context, apiBase, date string, athens *time.Location) (map[string][]model.Programme, error) {
	url := fmt.Sprintf("%s/get-events", apiBase)

	body := fmt.Sprintf("action=get_events&date=%s", date)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "greek-tv-scraper/1.0")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "https://www.digea.gr/el/tileoptikoi-stathmoi/ilektronikos-odigos-programmatos")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("digea: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("digea: unexpected status %d for %s — skipping", resp.StatusCode, url)
		return nil, nil
	}

	var events []DigeaEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		log.Printf("digea: JSON decode error: %v — skipping", err)
		return nil, nil
	}

	result := make(map[string][]model.Programme)
	for _, e := range events {
		epgID, ok := digeaToEPGID[e.ChannelID]
		if !ok {
			continue // skip channels not in our mapping
		}

		offset := "+0200"
		if e.UTCOffset == "3" {
			offset = "+0300"
		}

		start, err := time.Parse("2006-01-02 15:04:05 -0700", e.ActualTime+" "+offset)
		if err != nil {
			continue
		}
		end, err := time.Parse("2006-01-02 15:04:05 -0700", e.EndTime+" "+offset)
		if err != nil {
			continue
		}

		athensStart := start.In(athens)
		athensEnd := end.In(athens)

		result[epgID] = append(result[epgID], model.Programme{
			ChannelID:   epgID,
			Title:       e.Title,
			Description: e.LongSynopsis,
			StartTime:   start.UTC(),
			EndTime:     end.UTC(),
			StartLocal:  athensStart.Format("15:04"),
			EndLocal:    athensEnd.Format("15:04"),
			DateLocal:   athensStart.Format("2006-01-02"),
			Duration:    int(end.Sub(start).Minutes()),
		})
	}
	return result, nil
}
