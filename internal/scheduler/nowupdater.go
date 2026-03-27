package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/vpigadas/greek-tv-scraper/internal/metrics"
	"github.com/vpigadas/greek-tv-scraper/internal/model"
	"github.com/vpigadas/greek-tv-scraper/internal/registry"
	"github.com/vpigadas/greek-tv-scraper/internal/store"
)

// NowUpdater refreshes the pre-computed "now playing" cache every 60 seconds.
type NowUpdater struct {
	store  *store.Store
	athens *time.Location
	stop   chan struct{}
}

func NewNowUpdater(s *store.Store, athens *time.Location) *NowUpdater {
	return &NowUpdater{store: s, athens: athens, stop: make(chan struct{})}
}

func (u *NowUpdater) Start() {
	go u.run()
	log.Println("now-updater: started (interval: 60s)")
}

func (u *NowUpdater) Stop() {
	close(u.stop)
}

func (u *NowUpdater) run() {
	// Run immediately on start
	u.update()

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			u.update()
		case <-u.stop:
			return
		}
	}
}

func (u *NowUpdater) update() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now().UTC()

	// We need to check both today's and yesterday's broadcast day
	// because between 00:00-06:00 Athens time, programmes belong to the previous broadcast day.
	todayBD := store.BroadcastDate(now, u.athens)

	var results []model.NowPlaying
	for _, ch := range registry.Channels {
		progs, err := u.store.GetSchedule(ctx, ch.ID, todayBD)
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

	if err := u.store.SetNowPlaying(ctx, results); err != nil {
		log.Printf("now-updater: failed to write cache: %v", err)
		return
	}

	metrics.ChannelsLiveNow.Set(float64(len(results)))
}
