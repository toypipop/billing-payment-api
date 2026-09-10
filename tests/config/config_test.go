package config_test

import (
	"billing-payment-api/internal/config"
	"testing"
)

func TestInvalidConfig(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"DB_MAX_OPEN_CONNS", "0"}, {"DB_MAX_IDLE_CONNS", "-1"}, {"DB_MAX_IDLE_CONNS", "999"},
		{"DB_CONN_MAX_LIFETIME", "bad"}, {"DB_CONN_MAX_IDLE_TIME", "0s"}, {"REQUEST_TIMEOUT", "0s"},
		{"DB_STATEMENT_TIMEOUT", "-1s"}, {"DB_LOCK_TIMEOUT", "0s"},
	} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, err := config.Load(); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}
