package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port            string
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	XMLTVFeedURL    string
	DigeasAPIBase   string
	ERTScheduleBase string
	ScheduleTTL     time.Duration
	RefreshCron     string
	AthensLocation  *time.Location
}

func Load() (*Config, error) {
	athens, err := time.LoadLocation("Europe/Athens")
	if err != nil {
		athens = time.FixedZone("EET", 2*3600)
	}

	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "1"))

	return &Config{
		Port:            getEnv("PORT", "8082"),
		RedisAddr:       getEnv("REDIS_ADDR", "redis:6379"),
		RedisPassword:   getEnv("REDIS_PASSWORD", ""),
		RedisDB:         redisDB,
		XMLTVFeedURL:    getEnv("XMLTV_FEED_URL", "https://ext.greektv.app/epg/epg.xml.gz"),
		DigeasAPIBase:   getEnv("DIGEA_API_BASE", "https://www.digea.gr/el/api/epg"),
		ERTScheduleBase: getEnv("ERT_SCHEDULE_BASE", "https://www.ert.gr/tv/program"),
		ScheduleTTL:     parseDuration(getEnv("SCHEDULE_TTL", "6h")),
		RefreshCron:     getEnv("REFRESH_CRON", "0 */4 * * *"),
		AthensLocation:  athens,
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 6 * time.Hour
	}
	return d
}
