// Package logging construit le logger structuré utilisé par tout le bot.
//
// Contrairement au bot JS d'origine, dont le logger était un no-op en dehors de
// NODE_ENV=development, les logs sont toujours actifs ici : une erreur fatale au
// démarrage doit être visible dans journalctl.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// ParseLevel convertit "debug", "info", "warn" ou "error" en slog.Level.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("niveau de log inconnu %q (attendu debug|info|warn|error)", s)
	}
}

// New construit un logger texte. systemd horodate déjà chaque ligne du journal,
// on retire donc l'attribut "time" pour ne pas le dupliquer.
func New(w io.Writer, level slog.Level) *slog.Logger {
	h := slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})
	return slog.New(h)
}
