package api

import (
	"testing"
	"time"

	"github.com/vpigadas/greek-tv-scraper/internal/model"
)

func TestEnrichLive(t *testing.T) {
	now := time.Now().UTC()

	progs := []model.Programme{
		{Title: "Past Show", StartTime: now.Add(-2 * time.Hour), EndTime: now.Add(-1 * time.Hour)},
		{Title: "Current Show", StartTime: now.Add(-30 * time.Minute), EndTime: now.Add(30 * time.Minute)},
		{Title: "Future Show", StartTime: now.Add(1 * time.Hour), EndTime: now.Add(2 * time.Hour)},
	}

	enrichLive(progs)

	if progs[0].IsLive {
		t.Error("Past show should not be live")
	}
	if !progs[1].IsLive {
		t.Error("Current show should be live")
	}
	if progs[2].IsLive {
		t.Error("Future show should not be live")
	}
}

func TestEnrichLiveEmpty(t *testing.T) {
	var progs []model.Programme
	enrichLive(progs) // should not panic
}
