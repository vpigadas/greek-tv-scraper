package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/vpigadas/greek-tv-scraper/internal/config"
	"github.com/vpigadas/greek-tv-scraper/internal/model"
	"github.com/vpigadas/greek-tv-scraper/internal/registry"
	"github.com/vpigadas/greek-tv-scraper/internal/store"
)

type Handler struct {
	store  *store.Store
	cfg    *config.Config
	athens *time.Location
}

func NewHandler(s *store.Store, cfg *config.Config) *Handler {
	return &Handler{store: s, cfg: cfg, athens: cfg.AthensLocation}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/health", h.health)
	r.Get("/api/channels", h.listChannels)
	r.Get("/api/channels/{id}", h.getChannel)
	r.Get("/api/schedule/{id}", h.getSchedule)
	r.Get("/api/schedule/{id}/week", h.getScheduleWeek)
	r.Get("/api/schedule/{id}/{date}", h.getScheduleByDate)
	r.Get("/api/now", h.getNowPlaying)
	r.Get("/api/now/{id}", h.getNowPlayingForChannel)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	last, _ := h.store.GetLastRefresh(r.Context())
	respond(w, http.StatusOK, map[string]any{
		"status":       "ok",
		"last_refresh": last,
		"channels":     len(registry.Channels),
	})
}

func (h *Handler) listChannels(w http.ResponseWriter, r *http.Request) {
	respond(w, http.StatusOK, registry.Channels)
}

func (h *Handler) getChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ch := registry.ChannelByID(id)
	if ch == nil {
		respond(w, http.StatusNotFound, map[string]string{"error": "channel not found"})
		return
	}
	respond(w, http.StatusOK, ch)
}

// GET /api/schedule/{id}?date=2026-03-26
// Defaults to current broadcast day (06:00→06:00).
func (h *Handler) getSchedule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	date := r.URL.Query().Get("date")
	if date == "" {
		date = store.BroadcastDate(time.Now(), h.athens)
	}
	h.serveSchedule(w, r, id, date)
}

func (h *Handler) getScheduleByDate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	date := chi.URLParam(r, "date")
	h.serveSchedule(w, r, id, date)
}

// GET /api/schedule/{id}/week
// Returns 7 days of schedule (today's broadcast day + 6 days ahead) in a single response.
func (h *Handler) getScheduleWeek(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ch := registry.ChannelByID(id)
	if ch == nil {
		respond(w, http.StatusNotFound, map[string]string{"error": "channel not found"})
		return
	}

	todayBD := store.BroadcastDate(time.Now(), h.athens)
	todayDate, _ := time.ParseInLocation("2006-01-02", todayBD, h.athens)

	dates := make([]string, 7)
	for i := range dates {
		dates[i] = todayDate.AddDate(0, 0, i).Format("2006-01-02")
	}

	// Single Redis pipeline round-trip for all 7 days
	schedules, err := h.store.GetScheduleRange(r.Context(), id, dates)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "store error"})
		return
	}

	// Sort programmes within each day and enrich is_live
	for date, progs := range schedules {
		sort.Slice(progs, func(i, j int) bool {
			return progs[i].StartTime.Before(progs[j].StartTime)
		})
		enrichLive(progs)
		schedules[date] = progs
	}

	respond(w, http.StatusOK, map[string]any{
		"channel": ch,
		"from":    dates[0],
		"to":      dates[6],
		"days":    schedules,
	})
}

func (h *Handler) serveSchedule(w http.ResponseWriter, r *http.Request, channelID, date string) {
	ch := registry.ChannelByID(channelID)
	if ch == nil {
		respond(w, http.StatusNotFound, map[string]string{"error": "channel not found"})
		return
	}
	progs, err := h.store.GetSchedule(r.Context(), channelID, date)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "store error"})
		return
	}
	if progs == nil {
		respond(w, http.StatusNotFound, map[string]string{
			"error":   "no schedule data for this channel/date",
			"channel": channelID,
			"date":    date,
		})
		return
	}
	sort.Slice(progs, func(i, j int) bool {
		return progs[i].StartTime.Before(progs[j].StartTime)
	})
	enrichLive(progs)
	respond(w, http.StatusOK, map[string]any{
		"channel":         ch,
		"date":            date,
		"programme_count": len(progs),
		"programmes":      progs,
	})
}

// GET /api/now — reads pre-computed cache (single Redis GET).
func (h *Handler) getNowPlaying(w http.ResponseWriter, r *http.Request) {
	results, err := h.store.GetNowPlaying(r.Context())
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "store error"})
		return
	}
	if results == nil {
		results = []model.NowPlaying{}
	}
	respond(w, http.StatusOK, map[string]any{
		"count":   len(results),
		"now":     time.Now().UTC().Format(time.RFC3339),
		"playing": results,
	})
}

// GET /api/now/{id} — reads pre-computed cache and filters by channel.
func (h *Handler) getNowPlayingForChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ch := registry.ChannelByID(id)
	if ch == nil {
		respond(w, http.StatusNotFound, map[string]string{"error": "channel not found"})
		return
	}

	results, err := h.store.GetNowPlaying(r.Context())
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "store error"})
		return
	}

	for _, np := range results {
		if np.Channel.ID == id {
			respond(w, http.StatusOK, np)
			return
		}
	}

	respond(w, http.StatusOK, map[string]any{
		"channel": ch,
		"playing": nil,
		"message": "no programme found for current time",
	})
}

// enrichLive sets IsLive on each programme based on the current time.
func enrichLive(progs []model.Programme) {
	now := time.Now().UTC()
	for i := range progs {
		progs[i].IsLive = now.After(progs[i].StartTime) && now.Before(progs[i].EndTime)
	}
}

func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("respond: JSON encode error: %v", err)
	}
}
