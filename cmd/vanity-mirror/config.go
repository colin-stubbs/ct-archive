package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"filippo.io/sunlight"
)

const maxEntryFetchConcurrency = 512

// defaultBatchRetryBackoffs matches the historical fixed retry table in main.
func defaultBatchRetryBackoffs() []time.Duration {
	return []time.Duration{
		1 * time.Second,
		5 * time.Second,
		10 * time.Second,
		30 * time.Second,
		1 * time.Minute,
		2 * time.Minute,
		3 * time.Minute,
		5 * time.Minute,
		7 * time.Minute,
		10 * time.Minute,
	}
}

// mirrorConfig holds HTTP and batch behavior, from defaults and VANITY_MIRROR_* env.
type mirrorConfig struct {
	HTTPClientTimeout     time.Duration
	ResponseHeaderTimeout time.Duration // 0 disables (no header-only deadline beyond http.Client.Timeout)
	DialTimeout           time.Duration
	IdleConnTimeout       time.Duration
	EntryFetchConcurrency int
	MaxIdleConnsPerHost   int // 0 means derive as EntryFetchConcurrency+8
	BatchSize             int64
	BatchRetryBackoffs    []time.Duration
	HTTP429Delay          time.Duration
}

func defaultMirrorConfig() mirrorConfig {
	c := int64(sunlight.TileWidth * 128)
	return mirrorConfig{
		HTTPClientTimeout:     5 * time.Minute,
		ResponseHeaderTimeout: 60 * time.Second,
		DialTimeout:           30 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		EntryFetchConcurrency: 32,
		MaxIdleConnsPerHost:   0,
		BatchSize:             c,
		BatchRetryBackoffs:    append([]time.Duration(nil), defaultBatchRetryBackoffs()...),
		HTTP429Delay:          500 * time.Millisecond,
	}
}

func loadMirrorConfig() (mirrorConfig, error) {
	return loadMirrorConfigFromEnv(os.Getenv)
}

func loadMirrorConfigFromEnv(getenv func(string) string) (mirrorConfig, error) {
	cfg := defaultMirrorConfig()

	if s := getenv("VANITY_MIRROR_HTTP_CLIENT_TIMEOUT"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			return cfg, fmt.Errorf("VANITY_MIRROR_HTTP_CLIENT_TIMEOUT: %w", err)
		}
		if d <= 0 {
			return cfg, fmt.Errorf("VANITY_MIRROR_HTTP_CLIENT_TIMEOUT must be positive, got %s", s)
		}
		cfg.HTTPClientTimeout = d
	}

	if s := getenv("VANITY_MIRROR_RESPONSE_HEADER_TIMEOUT"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			return cfg, fmt.Errorf("VANITY_MIRROR_RESPONSE_HEADER_TIMEOUT: %w", err)
		}
		if d < 0 {
			return cfg, fmt.Errorf("VANITY_MIRROR_RESPONSE_HEADER_TIMEOUT must be >= 0, got %s", s)
		}
		cfg.ResponseHeaderTimeout = d
	}

	if s := getenv("VANITY_MIRROR_DIAL_TIMEOUT"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			return cfg, fmt.Errorf("VANITY_MIRROR_DIAL_TIMEOUT: %w", err)
		}
		if d <= 0 {
			return cfg, fmt.Errorf("VANITY_MIRROR_DIAL_TIMEOUT must be positive, got %s", s)
		}
		cfg.DialTimeout = d
	}

	if s := getenv("VANITY_MIRROR_IDLE_CONN_TIMEOUT"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			return cfg, fmt.Errorf("VANITY_MIRROR_IDLE_CONN_TIMEOUT: %w", err)
		}
		if d < 0 {
			return cfg, fmt.Errorf("VANITY_MIRROR_IDLE_CONN_TIMEOUT must be >= 0, got %s", s)
		}
		cfg.IdleConnTimeout = d
	}

	if s := getenv("VANITY_MIRROR_ENTRY_FETCH_CONCURRENCY"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			return cfg, fmt.Errorf("VANITY_MIRROR_ENTRY_FETCH_CONCURRENCY: %w", err)
		}
		if n < 1 {
			return cfg, fmt.Errorf("VANITY_MIRROR_ENTRY_FETCH_CONCURRENCY must be >= 1, got %d", n)
		}
		if n > maxEntryFetchConcurrency {
			return cfg, fmt.Errorf("VANITY_MIRROR_ENTRY_FETCH_CONCURRENCY must be <= %d, got %d", maxEntryFetchConcurrency, n)
		}
		cfg.EntryFetchConcurrency = n
	}

	if s := getenv("VANITY_MIRROR_MAX_IDLE_CONNS_PER_HOST"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			return cfg, fmt.Errorf("VANITY_MIRROR_MAX_IDLE_CONNS_PER_HOST: %w", err)
		}
		if n < 1 {
			return cfg, fmt.Errorf("VANITY_MIRROR_MAX_IDLE_CONNS_PER_HOST must be >= 1, got %d", n)
		}
		cfg.MaxIdleConnsPerHost = n
	}

	if s := getenv("VANITY_MIRROR_BATCH_SIZE"); s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return cfg, fmt.Errorf("VANITY_MIRROR_BATCH_SIZE: %w", err)
		}
		if n < 1 {
			return cfg, fmt.Errorf("VANITY_MIRROR_BATCH_SIZE must be >= 1, got %d", n)
		}
		cfg.BatchSize = n
	}

	if s := getenv("VANITY_MIRROR_BATCH_RETRY_BACKOFFS"); s != "" {
		parts := strings.Split(s, ",")
		var ds []time.Duration
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			d, err := time.ParseDuration(p)
			if err != nil {
				return cfg, fmt.Errorf("VANITY_MIRROR_BATCH_RETRY_BACKOFFS: token %q: %w", p, err)
			}
			if d < 0 {
				return cfg, fmt.Errorf("VANITY_MIRROR_BATCH_RETRY_BACKOFFS: negative duration %q", p)
			}
			ds = append(ds, d)
		}
		if len(ds) == 0 {
			return cfg, fmt.Errorf("VANITY_MIRROR_BATCH_RETRY_BACKOFFS: need at least one duration")
		}
		cfg.BatchRetryBackoffs = ds
	}

	if s := getenv("VANITY_MIRROR_HTTP_429_DELAY"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			return cfg, fmt.Errorf("VANITY_MIRROR_HTTP_429_DELAY: %w", err)
		}
		if d < 0 {
			return cfg, fmt.Errorf("VANITY_MIRROR_HTTP_429_DELAY must be >= 0, got %s", s)
		}
		cfg.HTTP429Delay = d
	}

	return cfg, nil
}

func (c mirrorConfig) effectiveMaxIdleConnsPerHost() int {
	if c.MaxIdleConnsPerHost > 0 {
		return c.MaxIdleConnsPerHost
	}
	return c.EntryFetchConcurrency + 8
}

func logMirrorConfig(logger *slog.Logger, c mirrorConfig) {
	var backoffStrs []string
	for _, d := range c.BatchRetryBackoffs {
		backoffStrs = append(backoffStrs, d.String())
	}
	logger.Info("vanity-mirror config",
		"http_client_timeout", c.HTTPClientTimeout.String(),
		"response_header_timeout", c.ResponseHeaderTimeout.String(),
		"dial_timeout", c.DialTimeout.String(),
		"idle_conn_timeout", c.IdleConnTimeout.String(),
		"entry_fetch_concurrency", c.EntryFetchConcurrency,
		"max_idle_conns_per_host", c.effectiveMaxIdleConnsPerHost(),
		"batch_size", c.BatchSize,
		"batch_retry_backoffs", strings.Join(backoffStrs, ","),
		"http_429_delay", c.HTTP429Delay.String(),
	)
}
