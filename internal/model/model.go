package model

import "time"

// Channel represents a Greek TV channel with metadata and live stream info.
type Channel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	LogoURL     string `json:"logo_url"`
	Group       string `json:"group"`
	LiveURL     string `json:"live_url"`
	LivePageURL string `json:"live_page_url"`
	EPGSource   string `json:"epg_source"`
}

// Programme represents a single TV show in the schedule.
type Programme struct {
	ChannelID   string    `json:"channel_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	StartLocal  string    `json:"start_local"`
	EndLocal    string    `json:"end_local"`
	DateLocal   string    `json:"date_local"`
	CoverURL    string    `json:"cover_url"`
	Category    string    `json:"category"`
	IsLive      bool      `json:"is_live"`
	Duration    int       `json:"duration_min"`
}

// ScheduleDay holds all programmes for a single channel on a single date.
type ScheduleDay struct {
	ChannelID  string      `json:"channel_id"`
	Date       string      `json:"date"`
	Programmes []Programme `json:"programmes"`
}

// NowPlaying is returned by the /now endpoint.
type NowPlaying struct {
	Channel   Channel   `json:"channel"`
	Programme Programme `json:"programme"`
	Progress  int       `json:"progress_pct"`
}
