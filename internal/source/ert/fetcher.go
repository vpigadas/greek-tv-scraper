package ert

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/vpigadas/greek-tv-scraper/internal/model"
)

var ertChannels = map[string]string{
	"ert1":      "ert1",
	"ert2":      "ert2",
	"ert3":      "ert3",
	"ertsports": "ertsports",
}

// FetchAll fetches schedule HTML for all ERT channels for a given date.
// date format: "2026-03-26"
func FetchAll(ctx context.Context, baseURL, date string, athens *time.Location) (map[string][]model.Programme, error) {
	result := make(map[string][]model.Programme)

	for slug, epgID := range ertChannels {
		url := fmt.Sprintf("%s/%s/?date=%s", baseURL, slug, date)
		progs, err := fetchChannel(ctx, url, epgID, date, athens)
		if err != nil {
			fmt.Printf("ert: fetch %s: %v\n", slug, err)
			continue
		}
		result[epgID] = progs
	}
	return result, nil
}

func fetchChannel(ctx context.Context, url, epgID, date string, athens *time.Location) ([]model.Programme, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "greek-tv-scraper/1.0")
	req.Header.Set("Accept-Language", "el-GR,el;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	// ERT schedule HTML structure (verified March 2026):
	//   <article class="broadcast" data-start-time="2026-03-26 06:45:00+0200" data-end-time="...">
	//     <div class="broadcast-time">06:45</div>
	//     <div class="broadcast-image"><img src="..."/></div>
	//     <div class="broadcast-content">
	//       <strong class="section-title">Title</strong>
	//       <span class="fs-ms">Description</span>
	//     </div>
	//   </article>
	var programmes []model.Programme
	parseArticles(doc, &programmes, epgID, athens)
	return programmes, nil
}

// parseArticles walks the HTML tree looking for <article class="broadcast"> elements.
func parseArticles(n *html.Node, programmes *[]model.Programme, epgID string, athens *time.Location) {
	if n.Type == html.ElementNode && n.Data == "article" {
		class := getAttr(n, "class")
		if strings.Contains(class, "broadcast") {
			prog := extractFromArticle(n, epgID, athens)
			if prog != nil {
				*programmes = append(*programmes, *prog)
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		parseArticles(c, programmes, epgID, athens)
	}
}

// extractFromArticle extracts a Programme from an <article class="broadcast"> element.
// Uses data-start-time and data-end-time attributes for precise timestamps.
func extractFromArticle(n *html.Node, epgID string, athens *time.Location) *model.Programme {
	// Parse timestamps from data attributes: "2026-03-26 06:45:00+0200"
	startStr := getAttr(n, "data-start-time")
	endStr := getAttr(n, "data-end-time")

	if startStr == "" {
		return nil
	}

	startTime, err := time.Parse("2006-01-02 15:04:05-0700", startStr)
	if err != nil {
		return nil
	}

	var endTime time.Time
	if endStr != "" {
		endTime, _ = time.Parse("2006-01-02 15:04:05-0700", endStr)
	}

	var title, description, coverURL, category string

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			class := getAttr(node, "class")
			switch {
			case strings.Contains(class, "broadcast-time"):
				// Skip — we use data-start-time instead
			case strings.Contains(class, "section-title"):
				title = strings.TrimSpace(innerText(node))
			case node.Data == "span" && strings.Contains(class, "fs-ms"):
				description = strings.TrimSpace(innerText(node))
			case node.Data == "strong" && strings.Contains(class, "fs-sm"):
				// Category line: <strong class="fs-sm"><span>Ψυχαγωγία</span>/<span>Εκπομπή</span></strong>
				category = strings.TrimSpace(innerText(node))
			case node.Data == "img" && strings.Contains(getAttr(node.Parent, "class"), "broadcast-image"):
				if src := getAttr(node, "src"); src != "" {
					coverURL = src
				}
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)

	if title == "" {
		return nil
	}

	athensStart := startTime.In(athens)
	duration := 0
	endLocal := ""
	if !endTime.IsZero() {
		duration = int(endTime.Sub(startTime).Minutes())
		endLocal = endTime.In(athens).Format("15:04")
	}

	return &model.Programme{
		ChannelID:   epgID,
		Title:       title,
		Description: description,
		StartTime:   startTime.UTC(),
		EndTime:     endTime.UTC(),
		StartLocal:  athensStart.Format("15:04"),
		EndLocal:    endLocal,
		DateLocal:   athensStart.Format("2006-01-02"),
		CoverURL:    coverURL,
		Category:    category,
		Duration:    duration,
	}
}

func getAttr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func innerText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}
