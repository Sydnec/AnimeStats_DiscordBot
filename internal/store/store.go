// Package store encapsule la persistance SQLite des abonnements.
//
// La table followers est reprise à l'identique du bot Node d'origine, afin
// qu'un fichier animestats.db existant soit utilisable sans migration — et
// qu'un retour arrière vers l'ancienne version reste possible. Les tables
// supplémentaires sont purement additives : l'ancien bot les ignore.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // pilote SQLite pur Go : permet CGO_ENABLED=0
)

// schema est exécuté à chaque ouverture, comme le faisait src/db.js.
const schema = `
CREATE TABLE IF NOT EXISTS followers (
  user_id TEXT PRIMARY KEY,
  anilist_username TEXT NOT NULL,
  freq_daily INTEGER DEFAULT 0,
  freq_monthly INTEGER DEFAULT 0,
  freq_yearly INTEGER DEFAULT 0,
  created_at INTEGER DEFAULT (strftime('%s','now'))
);

-- Clé/valeur interne : empreinte des commandes déjà publiées, etc.
CREATE TABLE IF NOT EXISTS meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

-- Cache des salons de messages privés : ouvrir un MP est une requête Discord
-- limitée à part, inutile de la refaire à chaque envoi.
CREATE TABLE IF NOT EXISTS dm_channels (
  user_id TEXT PRIMARY KEY,
  channel_id TEXT NOT NULL,
  updated_at INTEGER NOT NULL
);

-- Dernière exécution réussie de chaque tâche planifiée, pour rattraper un
-- envoi manqué si la machine était éteinte à l'heure prévue.
CREATE TABLE IF NOT EXISTS job_runs (
  job_key TEXT PRIMARY KEY,
  period TEXT NOT NULL,
  ran_at INTEGER NOT NULL
);`

// Store donne accès à la base des abonnés.
type Store struct {
	db *sql.DB
}

// Open ouvre (ou crée) la base au chemin donné.
//
// Le dossier parent est créé au besoin : son absence faisait planter le bot JS
// au démarrage sur une installation neuve.
func Open(ctx context.Context, path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("création du dossier %s : %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("ouverture de la base %s : %w", path, err)
	}
	// Une seule connexion : la charge est négligeable et cela élimine
	// définitivement les erreurs « database is locked ».
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connexion à la base %s : %w", path, err)
	}
	if _, err := db.ExecContext(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialisation du schéma : %w", err)
	}

	return &Store{db: db}, nil
}

// dsn construit l'URI SQLite.
//
// Le mode de journalisation est laissé tel quel : better-sqlite3 utilise le
// mode « delete » par défaut, et passer en WAL modifierait durablement
// l'en-tête du fichier tout en créant des fichiers -wal/-shm annexes. Seul
// busy_timeout est utile ici.
func dsn(path string) string {
	u := url.URL{Scheme: "file", Opaque: path}
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Set("_txlock", "immediate")
	u.RawQuery = q.Encode()
	return u.String()
}

// Close ferme la base.
func (s *Store) Close() error { return s.db.Close() }

// GetMeta lit une valeur interne. Le booléen indique sa présence.
func (s *Store) GetMeta(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("lecture de meta[%s] : %w", key, err)
	}
	return v, true, nil
}

// SetMeta écrit une valeur interne.
func (s *Store) SetMeta(ctx context.Context, key, value string) error {
	const q = `INSERT INTO meta (key, value) VALUES (?, ?)
	           ON CONFLICT(key) DO UPDATE SET value = excluded.value`
	if _, err := s.db.ExecContext(ctx, q, key, value); err != nil {
		return fmt.Errorf("écriture de meta[%s] : %w", key, err)
	}
	return nil
}
