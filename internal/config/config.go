package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	MaxOpenConns, MaxIdleConns                    int
	ConnMaxLifetime, ConnMaxIdleTime              time.Duration
	RequestTimeout, StatementTimeout, LockTimeout time.Duration
}

// Load fails at startup for invalid limits instead of silently disabling them.
func Load() (Config, error) {
	c := Config{MaxOpenConns: 20, MaxIdleConns: 10, ConnMaxLifetime: 30 * time.Minute, ConnMaxIdleTime: 5 * time.Minute, RequestTimeout: 10 * time.Second, StatementTimeout: 10 * time.Second, LockTimeout: 3 * time.Second}
	for _, setting := range []struct {
		name   string
		target *int
		min    int
	}{
		{"DB_MAX_OPEN_CONNS", &c.MaxOpenConns, 1}, {"DB_MAX_IDLE_CONNS", &c.MaxIdleConns, 0},
	} {
		if raw, ok := os.LookupEnv(setting.name); ok {
			value, err := strconv.Atoi(raw)
			if err != nil || value < setting.min {
				return c, fmt.Errorf("invalid %s", setting.name)
			}
			*setting.target = value
		}
	}
	if c.MaxIdleConns > c.MaxOpenConns {
		return c, fmt.Errorf("DB_MAX_IDLE_CONNS must not exceed DB_MAX_OPEN_CONNS")
	}
	for _, setting := range []struct {
		name   string
		target *time.Duration
	}{
		{"DB_CONN_MAX_LIFETIME", &c.ConnMaxLifetime}, {"DB_CONN_MAX_IDLE_TIME", &c.ConnMaxIdleTime},
		{"REQUEST_TIMEOUT", &c.RequestTimeout}, {"DB_STATEMENT_TIMEOUT", &c.StatementTimeout}, {"DB_LOCK_TIMEOUT", &c.LockTimeout},
	} {
		if raw, ok := os.LookupEnv(setting.name); ok {
			value, err := time.ParseDuration(raw)
			if err != nil || value < time.Millisecond {
				return c, fmt.Errorf("%s must be a duration of at least 1ms", setting.name)
			}
			*setting.target = value
		}
	}
	return c, nil
}
