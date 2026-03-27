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

func (sc *Scheduler) Start() error {
	// Initial refresh with timeout and panic protection
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC [scheduler initial refresh]: %v", r)
				metrics.PanicsTotal.WithLabelValues("scheduler").Inc()
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := sc.Refresh(ctx); err != nil {
			log.Printf("scheduler: initial refresh error: %v", err)
		}
	}()

	// Cron-scheduled refresh with panic protection
	_, err := sc.cron.AddFunc(sc.cfg.RefreshCron, func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("PANIC [scheduler cron refresh]: %v", r)
				metrics.PanicsTotal.WithLabelValues("scheduler").Inc()
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := sc.Refresh(ctx); err != nil {
			log.Printf("scheduler: refresh error: %v", err)
		}
	})
	if err != nil {
		return fmt.Errorf("scheduler: invalid cron expression %q: %w", sc.cfg.RefreshCron, err)
	}
	sc.cron.Start()
	log.Printf("scheduler: started (cron: %s)", sc.cfg.RefreshCron)
	return nil
}

func (sc *Scheduler) Stop() {
	sc.cron.Stop()
}

// Refresh fetches schedule data and stores it bucketed by broadcast day (06:00→06:00).
// Covers today-7 through today+7 (14 broadcast days).
func (sc *Scheduler) Refresh(ctx context.Context) error {
	refreshStart := time.Now()
	log.Println("scheduler: starting refresh")
	athens := sc.cfg.AthensLocation

	now := time.Now().In(athens)
	todayBroadcast := store.BroadcastDate(now, athens)

	// Build the 14-day date range
	todayDate, _ := time.ParseInLocation("2006-01-02", todayBroadcast, athens)
	dates := make([]string, 15) // -7 to +7
	for i := -7; i <= 7; i++ {
		dates[i+7] = todayDate.AddDate(0, 0, i).Format("2006-01-02")
	}

	// Step 1: Fetch XMLTV (primary, has multiple days)
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

	// Re-bucket XMLTV data by broadcast day (06:00 cutoff)
	xmltvByBroadcast := rebucketByBroadcastDay(xmltvData, athens)

	// Step 2: Fetch Digea events for today only (single batch POST)
	var digeaData map[string][]model.Programme
	log.Println("scheduler: fetching Digea events")
	digeaStart := time.Now()
	digeaData, err = digea.FetchAllEvents(ctx, sc.cfg.DigeasAPIBase, todayBroadcast, athens)
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

	// Step 3: For each channel × date, merge and store
	channelsWithData := 0
	totalProgrammes := 0
	programmesPerDay := make(map[string]int)            // date → total progs
	daysPerGroup := make(map[string]map[string]bool)    // group → set of dates with data

	for _, ch := range registry.Channels {
		channelHasData := false
		for _, date := range dates {
			isPast := date < todayBroadcast

			// Compose key: channelID + broadcastDate
			bucketKey := ch.ID + ":" + date
			var progs []model.Programme

			// XMLTV base data (already re-bucketed by broadcast day)
			if xmltvByBroadcast != nil {
				progs = append(progs, xmltvByBroadcast[bucketKey]...)
			}

			// Digea supplement for today
			if ch.EPGSource == "digea" && date == todayBroadcast && digeaData != nil {
				if fresh, ok := digeaData[ch.ID]; ok && len(fresh) > 0 {
					progs = rebucketChannel(fresh, date, athens)
				}
			}

			// Fill missing EndTimes
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
				programmesPerDay[date] += len(progs)
				if daysPerGroup[ch.Group] == nil {
					daysPerGroup[ch.Group] = make(map[string]bool)
				}
				daysPerGroup[ch.Group][date] = true
				if err := sc.store.SetSchedule(ctx, ch.ID, date, progs, isPast); err != nil {
					log.Printf("scheduler: redis store error for %s %s: %v", ch.ID, date, err)
				}
			}
		}
		if channelHasData {
			channelsWithData++
		}
	}

	metrics.ChannelsWithData.Set(float64(channelsWithData))
	metrics.ProgrammesStored.Set(float64(totalProgrammes))
	metrics.ChannelsTotal.Set(float64(len(registry.Channels)))

	// Data coverage metrics
	for group, dateSet := range daysPerGroup {
		metrics.ScheduleDaysAvailable.WithLabelValues(group).Set(float64(len(dateSet)))
	}
	for date, count := range programmesPerDay {
		metrics.ProgrammesPerDay.WithLabelValues(date).Set(float64(count))
	}
	metrics.RefreshDuration.WithLabelValues("total").Observe(time.Since(refreshStart).Seconds())
	metrics.RefreshTotal.WithLabelValues("success").Inc()
	metrics.RefreshLastSuccess.SetToCurrentTime()

	if err := sc.store.SetLastRefresh(ctx); err != nil {
		log.Printf("scheduler: failed to record last-refresh: %v", err)
	}

	log.Printf("scheduler: refresh complete — %d channels with data, %d programmes stored across %d days", channelsWithData, totalProgrammes, len(dates))
	return nil
}

// rebucketByBroadcastDay takes XMLTV data (keyed by channelID) and re-keys it
// as "channelID:broadcastDate" using the 06:00 cutoff.
func rebucketByBroadcastDay(data map[string][]model.Programme, athens *time.Location) map[string][]model.Programme {
	if data == nil {
		return nil
	}
	result := make(map[string][]model.Programme)
	for channelID, progs := range data {
		for _, p := range progs {
			bd := store.BroadcastDate(p.StartTime, athens)
			key := channelID + ":" + bd
			result[key] = append(result[key], p)
		}
	}
	return result
}

// rebucketChannel filters programmes for a single channel that belong to a specific broadcast day.
func rebucketChannel(progs []model.Programme, broadcastDate string, athens *time.Location) []model.Programme {
	var result []model.Programme
	for _, p := range progs {
		if store.BroadcastDate(p.StartTime, athens) == broadcastDate {
			result = append(result, p)
		}
	}
	return result
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
