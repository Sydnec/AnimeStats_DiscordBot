package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// DMChannel renvoie l'identifiant de salon privé mémorisé pour un utilisateur.
func (s *Store) DMChannel(ctx context.Context, userID string) (string, bool, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT channel_id FROM dm_channels WHERE user_id = ?`, userID).Scan(&id)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("lecture du salon privé de %s : %w", userID, err)
	}
	return id, id != "", nil
}

// SetDMChannel mémorise le salon privé d'un utilisateur.
func (s *Store) SetDMChannel(ctx context.Context, userID, channelID string) error {
	const q = `INSERT INTO dm_channels (user_id, channel_id, updated_at) VALUES (?, ?, ?)
	           ON CONFLICT(user_id) DO UPDATE SET channel_id = excluded.channel_id, updated_at = excluded.updated_at`
	if _, err := s.db.ExecContext(ctx, q, userID, channelID, time.Now().Unix()); err != nil {
		return fmt.Errorf("mémorisation du salon privé de %s : %w", userID, err)
	}
	return nil
}

// ForgetDMChannel oublie un salon devenu invalide.
func (s *Store) ForgetDMChannel(ctx context.Context, userID string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM dm_channels WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("oubli du salon privé de %s : %w", userID, err)
	}
	return nil
}

// LastJobPeriod renvoie la dernière période traitée par une tâche planifiée
// (par exemple "2025-09" pour le mensuel, "2024" pour l'annuel).
func (s *Store) LastJobPeriod(ctx context.Context, jobKey string) (string, bool, error) {
	var p string
	err := s.db.QueryRowContext(ctx, `SELECT period FROM job_runs WHERE job_key = ?`, jobKey).Scan(&p)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("lecture de la tâche %s : %w", jobKey, err)
	}
	return p, true, nil
}

// MarkJobRun enregistre la période que la tâche vient de traiter.
func (s *Store) MarkJobRun(ctx context.Context, jobKey, period string) error {
	const q = `INSERT INTO job_runs (job_key, period, ran_at) VALUES (?, ?, ?)
	           ON CONFLICT(job_key) DO UPDATE SET period = excluded.period, ran_at = excluded.ran_at`
	if _, err := s.db.ExecContext(ctx, q, jobKey, period, time.Now().Unix()); err != nil {
		return fmt.Errorf("enregistrement de la tâche %s : %w", jobKey, err)
	}
	return nil
}
