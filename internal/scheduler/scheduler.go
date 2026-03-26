package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/vpigadas/greek-tv-scraper/internal/config"
	"github.com/vpigadas/greek-tv-scraper/internal/metrics"
	"github.com/vpigadas/greek-tv-scraper/internal/model"
	"github.com/vpigadas/greek-tv-scraper/internal/registry"
	"github.com/vpigadas/greek-tv-scraper/internal/source/digea"
	"github.com/vpigadas/greek-tv-scraper/internal/source/xmltv"
	"github.com/vpigadas/greek-tv-scraper/internal/store"
)

type Scheduler struct {
	cfg   *config.Config
	store *store.Store
	cron  *cron.Cron
}

func New(cfg *config.Config, s *store.Store) *Scheduler {
	return &Scheduler{cfg: cfg, store: s, cron: cron.New()}
}

func (sc *Scheduler) Start() {
	go func() {
		if err := sc.Refresh(context.Background()); err != nil {
			log.Printf("scheduler: initial refresh error: %v", err)
		}
	}()

	_, err := sc.cron.AddFunc(sc.cfg.RefreshCron, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := sc.Refresh(ctx); err != nil {
			log.Printf("scheduler: refresh error: %v", err)
		}
	})
	if err != nil {
		log.Fatalf("scheduler: invalid cron expression %q: %v", sc.cfg.RefreshCron, err)
	}
	sc.cron.Start()
	log.Printf("scheduler: started (cron: %s)", sc.cfg.RefreshCron)
}

func (sc *Scheduler) Stop() {
	sc.cron.Stop()
}

// Refresh fetches fresh schedule data for today and tomorrow.
func (sc *Scheduler) Refresh(ctx context.Context) error {
	refreshStart := time.Now()
	log.Println("scheduler: starting refresh")
	athens := sc.cfg.AthensLocation

	today := time.Now().In(athens).Format("2006-01-02")
	tomorrow := time.Now().In(athens).Add(24 * time.Hour).Format("2006-01-02")

	// Step 1: Fetch XMLTV
	log.Println("scheduler: fetching XMLTV feed")
	xmltvStart := time.Now()
	xmltvData, err := xmltv.Fetch(ctx, sc.cfg.XMLTVFeedURL, athens)
	metrics.RefreshDuration.WithLabelValues("xmltv").Observe(time.Since(xmltvStart).Seconds())
	if err != nil {
		log.Printf("scheduler: XMLTV fetch failed: %v (will use cached data)", err)
		metrics.SourceFetchErrors.WithLabelValues("xmltv").Inc()
	} else {
		xmltvCount := 0
		for _, progs := range xmltvData {
			xmltvCount += len(progs)
		}
		metrics.ProgrammesFetched.WithLabelValues("xmltv").Set(float64(xmltvCount))
	}

	// Step 2: Fetch Digea events for today
	var digeaData map[string][]model.Programme
	log.Println("scheduler: fetching Digea events")
	digeaStart := time.Now()
	digeaData, err = digea.FetchAllEvents(ctx, sc.cfg.DigeasAPIBase, today, athens)
	metrics.RefreshDuration.WithLabelValues("digea").Observe(time.Since(digeaStart).Seconds())
	if err != nil {
		log.Printf("scheduler: Digea fetch failed: %v — will use XMLTV data", err)
		metrics.SourceFetchErrors.WithLabelValues("digea").Inc()
	} else if digeaData != nil {
		digeaCount := 0
		for _, progs := range digeaData {
			digeaCount += len(progs)
		}
		metrics.ProgrammesFetched.WithLabelValues("digea").Set(float64(digeaCount))
	}

	// Step 3: For each channel in registry, merge and store
	channelsWithData := 0
	totalProgrammes := 0

	for _, ch := range registry.Channels {
		channelHasData := false
		for _, date := range []string{today, tomorrow} {
			var progs []model.Programme

			if xmltvData != nil {
				for _, p := range xmltvData[ch.ID] {
					if p.DateLocal == date {
						progs = append(progs, p)
					}
				}
			}

			if ch.EPGSource == "digea" && date == today && digeaData != nil {
				if fresh, ok := digeaData[ch.ID]; ok && len(fresh) > 0 {
					progs = fresh
				}
			}

			// Fill missing EndTimes from next programme's StartTime
			for i := range progs {
				if progs[i].EndTime.IsZero() && i < len(progs)-1 {
					progs[i].EndTime = progs[i+1].StartTime
					progs[i].EndLocal = progs[i+1].StartLocal
					progs[i].Duration = int(progs[i].EndTime.Sub(progs[i].StartTime).Minutes())
				}
			}

			if len(progs) > 0 {
				channelHasData = true
				totalProgrammes += len(progs)
				if err := sc.store.SetSchedule(ctx, ch.ID, date, progs); err != nil {
					log.Printf("scheduler: redis store error for %s %s: %v", ch.ID, date, err)
				}
			}
		}
		if channelHasData {
			channelsWithData++
		}
	}

	// Record metrics
	metrics.ChannelsWithData.Set(float64(channelsWithData))
	metrics.ProgrammesStored.Set(float64(totalProgrammes))
	metrics.ChannelsTotal.Set(float64(len(registry.Channels)))
	metrics.RefreshDuration.WithLabelValues("total").Observe(time.Since(refreshStart).Seconds())
	metrics.RefreshTotal.WithLabelValues("success").Inc()
	metrics.RefreshLastSuccess.SetToCurrentTime()

	if err := sc.store.SetLastRefresh(ctx); err != nil {
		log.Printf("scheduler: failed to record last-refresh: %v", err)
	}

	log.Printf("scheduler: refresh complete — %d channels with data, %d programmes stored", channelsWithData, totalProgrammes)
	return nil
}

func (sc *Scheduler) RefreshStatus(ctx context.Context) string {
	last, err := sc.store.GetLastRefresh(ctx)
	if err != nil || last == "" {
		return "never"
	}
	t, err := time.Parse(time.RFC3339, last)
	if err != nil {
		return last
	}
	ago := time.Since(t).Round(time.Minute)
	return fmt.Sprintf("%s (%s ago)", last, ago)
}
