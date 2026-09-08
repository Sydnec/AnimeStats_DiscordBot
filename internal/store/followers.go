package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Sydnec/AnimeStats_DiscordBot/internal/mask"
)

// Frequency identifie une colonne de fréquence de la table followers.
type Frequency string

const (
	Daily   Frequency = "daily"
	Monthly Frequency = "monthly"
	Yearly  Frequency = "yearly"
)

func (f Frequency) column() (string, error) {
	switch f {
	case Daily:
		return "freq_daily", nil
	case Monthly:
		return "freq_monthly", nil
	case Yearly:
		return "freq_yearly", nil
	default:
		return "", fmt.Errorf("fréquence inconnue %q", string(f))
	}
}

// Follower est une ligne de la table followers.
type Follower struct {
	UserID          string
	AniListUsername string
	Freqs           mask.Freqs
	CreatedAt       int64
}

// ErrNotFound est renvoyée par Get quand l'utilisateur n'est pas abonné.
var ErrNotFound = errors.New("abonné introuvable")

const selectColumns = `user_id, anilist_username, freq_daily, freq_monthly, freq_yearly, COALESCE(created_at, 0)`

// Upsert crée ou met à jour l'abonnement d'un utilisateur.
//
// Les fréquences sont écrites telles quelles : la fusion avec l'état existant
// (sémantique du masque '*') relève de l'appelant, via mask.Mask.Apply.
func (s *Store) Upsert(ctx context.Context, userID, anilistUsername string, freqs mask.Freqs) error {
	const q = `
INSERT INTO followers (user_id, anilist_username, freq_daily, freq_monthly, freq_yearly)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(user_id) DO UPDATE SET
  anilist_username = excluded.anilist_username,
  freq_daily       = excluded.freq_daily,
  freq_monthly     = excluded.freq_monthly,
  freq_yearly      = excluded.freq_yearly`

	// Le récap quotidien est désactivé : la colonne est conservée pour
	// compatibilité de schéma mais toujours écrite à 0.
	_, err := s.db.ExecContext(ctx, q, userID, anilistUsername, 0, boolToInt(freqs.Monthly), boolToInt(freqs.Yearly))
	if err != nil {
		return fmt.Errorf("enregistrement de l'abonné %s : %w", userID, err)
	}
	return nil
}

// Remove supprime l'abonnement. Le booléen indique si une ligne existait.
func (s *Store) Remove(ctx context.Context, userID string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM followers WHERE user_id = ?`, userID)
	if err != nil {
		return false, fmt.Errorf("suppression de l'abonné %s : %w", userID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("suppression de l'abonné %s : %w", userID, err)
	}
	return n > 0, nil
}

// Get renvoie l'abonnement d'un utilisateur, ou ErrNotFound.
func (s *Store) Get(ctx context.Context, userID string) (Follower, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+selectColumns+` FROM followers WHERE user_id = ?`, userID)

	f, err := scanFollower(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Follower{}, ErrNotFound
	}
	if err != nil {
		return Follower{}, fmt.Errorf("lecture de l'abonné %s : %w", userID, err)
	}
	return f, nil
}

// ListByFrequency renvoie les abonnés dont la fréquence demandée est active.
func (s *Store) ListByFrequency(ctx context.Context, freq Frequency) ([]Follower, error) {
	col, err := freq.column()
	if err != nil {
		return nil, err
	}
	// col provient d'une liste fermée : aucune donnée utilisateur n'est
	// interpolée dans la requête.
	return s.query(ctx, `SELECT `+selectColumns+` FROM followers WHERE `+col+` = 1 ORDER BY user_id`)
}

// ListAll renvoie tous les abonnés.
func (s *Store) ListAll(ctx context.Context) ([]Follower, error) {
	return s.query(ctx, `SELECT `+selectColumns+` FROM followers ORDER BY user_id`)
}

func (s *Store) query(ctx context.Context, q string, args ...any) ([]Follower, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("lecture des abonnés : %w", err)
	}
	defer rows.Close()

	var out []Follower
	for rows.Next() {
		f, err := scanFollower(rows)
		if err != nil {
			return nil, fmt.Errorf("lecture des abonnés : %w", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lecture des abonnés : %w", err)
	}
	return out, nil
}

// scanner couvre *sql.Row comme *sql.Rows.
type scanner interface{ Scan(dest ...any) error }

func scanFollower(sc scanner) (Follower, error) {
	var (
		f                      Follower
		daily, monthly, yearly sql.NullInt64
	)
	if err := sc.Scan(&f.UserID, &f.AniListUsername, &daily, &monthly, &yearly, &f.CreatedAt); err != nil {
		return Follower{}, err
	}
	f.Freqs = mask.Freqs{
		Daily:   daily.Int64 == 1,
		Monthly: monthly.Int64 == 1,
		Yearly:  yearly.Int64 == 1,
	}
	return f, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
