// Package config resolves runtime settings from flags, falling back to environment variables.
package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr         string
	DBPath       string
	Location     *time.Location
	CookieSecure bool
}

func Load(args []string) (Config, error) {
	fs := flag.NewFlagSet("bp-tracker", flag.ContinueOnError)
	addr := fs.String("addr", envOr("BP_ADDR", ":8080"), "listen address")
	dbPath := fs.String("db", envOr("BP_DB", "./data/bp.db"), "SQLite database file")
	tz := fs.String("tz", envOr("BP_TZ", "Asia/Shanghai"), "time zone used to split days into am/pm")
	secure := fs.Bool("cookie-secure", envBool("BP_COOKIE_SECURE", false), "mark session cookie Secure (enable behind HTTPS)")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	loc, err := time.LoadLocation(*tz)
	if err != nil {
		return Config{}, fmt.Errorf("load time zone %q: %w", *tz, err)
	}
	return Config{Addr: *addr, DBPath: *dbPath, Location: loc, CookieSecure: *secure}, nil
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
