package api

import (
	"encoding/json"
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

func (h *Handler) getSchedule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().In(h.athens).Format("2006-01-02")
	}
	h.serveSchedule(w, r, id, date)
}

func (h *Handler) getScheduleByDate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	date := chi.URLParam(r, "date")
	h.serveSchedule(w, r, id, date)
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

func (h *Handler) getNowPlaying(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	date := now.In(h.athens).Format("2006-01-02")

	var results []model.NowPlaying
	for _, ch := range registry.Channels {
		progs, err := h.store.GetSchedule(r.Context(), ch.ID, date)
		if err != nil || len(progs) == 0 {
			continue
		}
		for _, p := range progs {
			if now.After(p.StartTime) && now.Before(p.EndTime) {
				progress := 0
				total := int(p.EndTime.Sub(p.StartTime).Seconds())
				if total > 0 {
					elapsed := int(now.Sub(p.StartTime).Seconds())
					progress = (elapsed * 100) / total
				}
				results = append(results, model.NowPlaying{
					Channel:   ch,
					Programme: p,
					Progress:  progress,
				})
				break
			}
		}
	}
	respond(w, http.StatusOK, map[string]any{
		"count":   len(results),
		"now":     now.Format(time.RFC3339),
		"playing": results,
	})
}

func (h *Handler) getNowPlayingForChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ch := registry.ChannelByID(id)
	if ch == nil {
		respond(w, http.StatusNotFound, map[string]string{"error": "channel not found"})
		return
	}

	now := time.Now().UTC()
	date := now.In(h.athens).Format("2006-01-02")
	progs, err := h.store.GetSchedule(r.Context(), id, date)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "store error"})
		return
	}

	for _, p := range progs {
		if now.After(p.StartTime) && now.Before(p.EndTime) {
			progress := 0
			total := int(p.EndTime.Sub(p.StartTime).Seconds())
			if total > 0 {
				elapsed := int(now.Sub(p.StartTime).Seconds())
				progress = (elapsed * 100) / total
			}
			respond(w, http.StatusOK, model.NowPlaying{
				Channel:   *ch,
				Programme: p,
				Progress:  progress,
			})
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
	_ = json.NewEncoder(w).Encode(body)
}
