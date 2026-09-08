// Package config rassemble la configuration du bot, lue depuis l'environnement.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Sydnec/AnimeStats_DiscordBot/internal/logging"
)

// Valeurs par défaut. DefaultDBPath vise l'emplacement utilisé par l'unit
// systemd ; en développement, DB_PATH pointe plutôt vers ./data/animestats.db.
const (
	DefaultDBPath      = "/var/lib/animestats/animestats.db"
	DefaultTimezone    = "Europe/Paris"
	DefaultOpEdMinutes = 3.0
	DefaultMaxPages    = 20
	DefaultMonthlyCron = "0 10 1 * *"
	DefaultYearlyCron  = "0 12 1 1 *"
)

// Config est la configuration résolue et validée du bot.
type Config struct {
	DiscordToken string
	// DevGuildID, s'il est renseigné, enregistre les commandes sur cette seule
	// guilde (propagation immédiate) plutôt qu'en global (jusqu'à une heure).
	DevGuildID string

	DBPath   string
	Location *time.Location

	OpEdMinutes     float64
	AniListMaxPages int
	AniListCacheTTL time.Duration

	LogLevel          slog.Level
	SendRecapOnFollow bool

	MonthlyCron string
	YearlyCron  string
}

// Load lit .env (s'il existe) puis l'environnement, et valide le résultat.
func Load() (Config, error) {
	if err := LoadDotEnv(".env"); err != nil {
		return Config{}, fmt.Errorf("lecture du .env : %w", err)
	}

	cfg := Config{
		DiscordToken: strings.TrimSpace(os.Getenv("DISCORD_TOKEN")),
		DevGuildID:   strings.TrimSpace(os.Getenv("DEV_GUILD_ID")),
		DBPath:       envString("DB_PATH", DefaultDBPath),
		MonthlyCron:  envString("MONTHLY_CRON", DefaultMonthlyCron),
		YearlyCron:   envString("YEARLY_CRON", DefaultYearlyCron),
	}

	if cfg.DiscordToken == "" {
		return Config{}, fmt.Errorf("DISCORD_TOKEN est requis")
	}

	tz := envString("TZ", DefaultTimezone)
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return Config{}, fmt.Errorf("fuseau horaire TZ=%q invalide : %w", tz, err)
	}
	cfg.Location = loc

	if cfg.OpEdMinutes, err = envFloat("OP_ED_MINUTES", DefaultOpEdMinutes); err != nil {
		return Config{}, err
	}
	if cfg.OpEdMinutes < 0 {
		return Config{}, fmt.Errorf("OP_ED_MINUTES ne peut pas être négatif (reçu %v)", cfg.OpEdMinutes)
	}

	if cfg.AniListMaxPages, err = envInt("ANILIST_MAX_PAGES", DefaultMaxPages); err != nil {
		return Config{}, err
	}
	if cfg.AniListMaxPages <= 0 {
		return Config{}, fmt.Errorf("ANILIST_MAX_PAGES doit être strictement positif (reçu %d)", cfg.AniListMaxPages)
	}

	if cfg.AniListCacheTTL, err = envDuration("ANILIST_CACHE_TTL", 0); err != nil {
		return Config{}, err
	}

	if cfg.LogLevel, err = logging.ParseLevel(os.Getenv("LOG_LEVEL")); err != nil {
		return Config{}, err
	}

	if cfg.SendRecapOnFollow, err = envBool("SEND_RECAP_ON_FOLLOW", true); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func envString(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s=%q n'est pas un entier valide", key, raw)
	}
	return v, nil
}

func envFloat(key string, def float64) (float64, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s=%q n'est pas un nombre valide", key, raw)
	}
	return v, nil
}

func envBool(key string, def bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s=%q n'est pas un booléen valide", key, raw)
	}
	return v, nil
}

// envDuration accepte une durée Go ("5m", "30s") ou un nombre nu de secondes,
// pour rester compatible avec l'ancien ANILIST_CACHE_TTL_SECONDS.
func envDuration(key string, def time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def, nil
	}
	if secs, err := strconv.Atoi(raw); err == nil {
		if secs < 0 {
			return 0, fmt.Errorf("%s=%q ne peut pas être négatif", key, raw)
		}
		return time.Duration(secs) * time.Second, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s=%q n'est pas une durée valide (ex: 5m, 90s, ou un nombre de secondes)", key, raw)
	}
	if d < 0 {
		return 0, fmt.Errorf("%s=%q ne peut pas être négatif", key, raw)
	}
	return d, nil
}
