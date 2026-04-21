package main

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"filippo.io/sunlight"
)

func TestLoadMirrorConfigFromEnv(t *testing.T) {
	t.Parallel()

	defaultBatch := int64(sunlight.TileWidth * 128)
	defaultBackoff := defaultBatchRetryBackoffs()

	t.Run("empty env uses defaults", func(t *testing.T) {
		t.Parallel()
		env := map[string]string{}
		got, err := loadMirrorConfigFromEnv(lookupMap(env))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.HTTPClientTimeout != 5*time.Minute {
			t.Errorf("HTTPClientTimeout = %v, want 5m", got.HTTPClientTimeout)
		}
		if got.ResponseHeaderTimeout != 60*time.Second {
			t.Errorf("ResponseHeaderTimeout = %v, want 60s", got.ResponseHeaderTimeout)
		}
		if got.DialTimeout != 30*time.Second || got.IdleConnTimeout != 90*time.Second {
			t.Errorf("dial/idle: dial=%v idle=%v", got.DialTimeout, got.IdleConnTimeout)
		}
		if got.EntryFetchConcurrency != 32 || got.MaxIdleConnsPerHost != 0 {
			t.Errorf("concurrency/maxIdle: %d / %d", got.EntryFetchConcurrency, got.MaxIdleConnsPerHost)
		}
		if got.BatchSize != defaultBatch {
			t.Errorf("BatchSize = %d, want %d", got.BatchSize, defaultBatch)
		}
		if got.HTTP429Delay != 500*time.Millisecond {
			t.Errorf("HTTP429Delay = %v", got.HTTP429Delay)
		}
		if !reflect.DeepEqual(got.BatchRetryBackoffs, defaultBackoff) {
			t.Errorf("BatchRetryBackoffs = %#v, want %#v", got.BatchRetryBackoffs, defaultBackoff)
		}
		if got.effectiveMaxIdleConnsPerHost() != got.EntryFetchConcurrency+8 {
			t.Errorf("effective max idle = %d", got.effectiveMaxIdleConnsPerHost())
		}
	})

	t.Run("concurrency override", func(t *testing.T) {
		t.Parallel()
		env := map[string]string{
			"VANITY_MIRROR_ENTRY_FETCH_CONCURRENCY": "8",
		}
		got, err := loadMirrorConfigFromEnv(lookupMap(env))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.EntryFetchConcurrency != 8 {
			t.Fatalf("concurrency = %d", got.EntryFetchConcurrency)
		}
		if got.effectiveMaxIdleConnsPerHost() != 8+8 {
			t.Errorf("effective max idle = %d, want 16", got.effectiveMaxIdleConnsPerHost())
		}
	})

	t.Run("max idle override", func(t *testing.T) {
		t.Parallel()
		env := map[string]string{
			"VANITY_MIRROR_MAX_IDLE_CONNS_PER_HOST": "99",
		}
		got, err := loadMirrorConfigFromEnv(lookupMap(env))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.effectiveMaxIdleConnsPerHost() != 99 {
			t.Errorf("effective max idle = %d", got.effectiveMaxIdleConnsPerHost())
		}
	})

	t.Run("invalid http client timeout", func(t *testing.T) {
		t.Parallel()
		env := map[string]string{"VANITY_MIRROR_HTTP_CLIENT_TIMEOUT": "not-a-duration"}
		_, err := loadMirrorConfigFromEnv(lookupMap(env))
		if err == nil || !strings.Contains(err.Error(), "VANITY_MIRROR_HTTP_CLIENT_TIMEOUT") {
			t.Fatalf("want error mentioning env, got %v", err)
		}
	})

	t.Run("non-positive http client timeout", func(t *testing.T) {
		t.Parallel()
		env := map[string]string{"VANITY_MIRROR_HTTP_CLIENT_TIMEOUT": "0"}
		_, err := loadMirrorConfigFromEnv(lookupMap(env))
		if err == nil || !strings.Contains(err.Error(), "positive") {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("invalid batch retry backoffs", func(t *testing.T) {
		t.Parallel()
		env := map[string]string{"VANITY_MIRROR_BATCH_RETRY_BACKOFFS": "1s,bogus"}
		_, err := loadMirrorConfigFromEnv(lookupMap(env))
		if err == nil || !strings.Contains(err.Error(), "VANITY_MIRROR_BATCH_RETRY_BACKOFFS") {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("response header timeout zero disables", func(t *testing.T) {
		t.Parallel()
		env := map[string]string{"VANITY_MIRROR_RESPONSE_HEADER_TIMEOUT": "0"}
		got, err := loadMirrorConfigFromEnv(lookupMap(env))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ResponseHeaderTimeout != 0 {
			t.Fatalf("ResponseHeaderTimeout = %v, want 0", got.ResponseHeaderTimeout)
		}
	})

	t.Run("batch retry backoffs two tokens", func(t *testing.T) {
		t.Parallel()
		env := map[string]string{"VANITY_MIRROR_BATCH_RETRY_BACKOFFS": "2s, 3s"}
		got, err := loadMirrorConfigFromEnv(lookupMap(env))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []time.Duration{2 * time.Second, 3 * time.Second}
		if !reflect.DeepEqual(got.BatchRetryBackoffs, want) {
			t.Fatalf("got %#v, want %#v", got.BatchRetryBackoffs, want)
		}
	})
}

func lookupMap(m map[string]string) func(string) string {
	return func(k string) string {
		return m[k]
	}
}
