package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestProgrammeJSON(t *testing.T) {
	p := Programme{
		ChannelID:   "ant1",
		Title:       "Test Show",
		Description: "A test programme",
		StartTime:   time.Date(2026, 3, 26, 9, 0, 0, 0, time.UTC),
		EndTime:     time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC),
		StartLocal:  "11:00",
		EndLocal:    "12:00",
		DateLocal:   "2026-03-26",
		Category:    "Entertainment",
		IsLive:      true,
		Duration:    60,
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Programme
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ChannelID != p.ChannelID {
		t.Errorf("ChannelID = %q, want %q", got.ChannelID, p.ChannelID)
	}
	if got.Title != p.Title {
		t.Errorf("Title = %q, want %q", got.Title, p.Title)
	}
	if got.Duration != 60 {
		t.Errorf("Duration = %d, want 60", got.Duration)
	}
	if !got.IsLive {
		t.Error("IsLive = false, want true")
	}
}

func TestNowPlayingProgress(t *testing.T) {
	np := NowPlaying{
		Channel: Channel{ID: "ert1", Name: "ERT1"},
		Programme: Programme{
			Title:     "News",
			StartTime: time.Now().UTC().Add(-30 * time.Minute),
			EndTime:   time.Now().UTC().Add(30 * time.Minute),
		},
		Progress: 50,
	}

	if np.Progress != 50 {
		t.Errorf("Progress = %d, want 50", np.Progress)
	}
}
