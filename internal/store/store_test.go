package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/Sydnec/AnimeStats_DiscordBot/internal/mask"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	// Le chemin passe par un sous-dossier inexistant : Open doit le créer.
	path := filepath.Join(t.TempDir(), "nested", "animestats.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open : %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenCreatesParentDirectory(t *testing.T) {
	s := newStore(t)
	if _, err := s.ListAll(context.Background()); err != nil {
		t.Fatalf("ListAll sur base neuve : %v", err)
	}
}

func TestUpsertGetRemove(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	if _, err := s.Get(ctx, "42"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get sur un inconnu = %v, attendu ErrNotFound", err)
	}

	if err := s.Upsert(ctx, "42", "Sydnec", mask.Freqs{Monthly: true}); err != nil {
		t.Fatalf("Upsert : %v", err)
	}

	got, err := s.Get(ctx, "42")
	if err != nil {
		t.Fatalf("Get : %v", err)
	}
	if got.UserID != "42" || got.AniListUsername != "Sydnec" {
		t.Errorf("Get = %+v", got)
	}
	if want := (mask.Freqs{Monthly: true}); got.Freqs != want {
		t.Errorf("Freqs = %+v, attendu %+v", got.Freqs, want)
	}
	if got.CreatedAt == 0 {
		t.Error("created_at aurait dû être renseigné par le défaut du schéma")
	}

	// Un second Upsert écrase pseudo et fréquences sans dupliquer la ligne.
	if err := s.Upsert(ctx, "42", "Autre", mask.Freqs{Yearly: true}); err != nil {
		t.Fatalf("Upsert (mise à jour) : %v", err)
	}
	got, err = s.Get(ctx, "42")
	if err != nil {
		t.Fatalf("Get après mise à jour : %v", err)
	}
	if got.AniListUsername != "Autre" {
		t.Errorf("pseudo = %q, attendu %q", got.AniListUsername, "Autre")
	}
	if want := (mask.Freqs{Yearly: true}); got.Freqs != want {
		t.Errorf("Freqs = %+v, attendu %+v", got.Freqs, want)
	}

	all, err := s.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll : %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("ListAll = %d lignes, attendu 1", len(all))
	}

	removed, err := s.Remove(ctx, "42")
	if err != nil {
		t.Fatalf("Remove : %v", err)
	}
	if !removed {
		t.Error("Remove aurait dû signaler une suppression")
	}
	removed, err = s.Remove(ctx, "42")
	if err != nil {
		t.Fatalf("Remove (seconde fois) : %v", err)
	}
	if removed {
		t.Error("Remove sur un inconnu aurait dû renvoyer false")
	}
}

func TestListByFrequency(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	must := func(userID, name string, f mask.Freqs) {
		t.Helper()
		if err := s.Upsert(ctx, userID, name, f); err != nil {
			t.Fatalf("Upsert(%s) : %v", userID, err)
		}
	}
	must("1", "a", mask.Freqs{Monthly: true})
	must("2", "b", mask.Freqs{Yearly: true})
	must("3", "c", mask.Freqs{Monthly: true, Yearly: true})
	must("4", "d", mask.Freqs{})

	cases := map[Frequency][]string{
		Monthly: {"1", "3"},
		Yearly:  {"2", "3"},
		Daily:   nil, // toujours forcé à 0
	}
	for freq, want := range cases {
		rows, err := s.ListByFrequency(ctx, freq)
		if err != nil {
			t.Fatalf("ListByFrequency(%s) : %v", freq, err)
		}
		var got []string
		for _, r := range rows {
			got = append(got, r.UserID)
		}
		if len(got) != len(want) {
			t.Fatalf("ListByFrequency(%s) = %v, attendu %v", freq, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("ListByFrequency(%s) = %v, attendu %v", freq, got, want)
				break
			}
		}
	}

	if _, err := s.ListByFrequency(ctx, Frequency("weekly")); err == nil {
		t.Error("une fréquence inconnue aurait dû être rejetée")
	}
}

// TestLegacyDatabaseIsReadable reproduit une base créée par l'ancien bot
// (better-sqlite3, src/db.js) et vérifie qu'elle est exploitable telle quelle.
func TestLegacyDatabaseIsReadable(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy.db")

	legacy, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("ouverture de la base héritée : %v", err)
	}
	const legacySchema = `
CREATE TABLE IF NOT EXISTS followers (
  user_id TEXT PRIMARY KEY,
  anilist_username TEXT NOT NULL,
  freq_daily INTEGER DEFAULT 0,
  freq_monthly INTEGER DEFAULT 0,
  freq_yearly INTEGER DEFAULT 0,
  created_at INTEGER DEFAULT (strftime('%s','now'))
);`
	if _, err := legacy.ExecContext(ctx, legacySchema); err != nil {
		t.Fatalf("création du schéma hérité : %v", err)
	}
	// Un abonné existant, avec freq_daily à 1 comme le permettaient les
	// premières versions du bot.
	if _, err := legacy.ExecContext(ctx,
		`INSERT INTO followers (user_id, anilist_username, freq_daily, freq_monthly, freq_yearly) VALUES (?,?,?,?,?)`,
		"999", "AncienPseudo", 1, 1, 0); err != nil {
		t.Fatalf("insertion héritée : %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("fermeture de la base héritée : %v", err)
	}

	s, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open sur base héritée : %v", err)
	}
	defer s.Close()

	got, err := s.Get(ctx, "999")
	if err != nil {
		t.Fatalf("Get sur base héritée : %v", err)
	}
	if got.AniListUsername != "AncienPseudo" {
		t.Errorf("pseudo = %q", got.AniListUsername)
	}
	if want := (mask.Freqs{Daily: true, Monthly: true}); got.Freqs != want {
		t.Errorf("Freqs = %+v, attendu %+v", got.Freqs, want)
	}

	// Une mise à jour par le nouveau bot remet freq_daily à 0.
	if err := s.Upsert(ctx, "999", "AncienPseudo", mask.Freqs{Daily: true, Monthly: true}); err != nil {
		t.Fatalf("Upsert : %v", err)
	}
	got, err = s.Get(ctx, "999")
	if err != nil {
		t.Fatalf("Get : %v", err)
	}
	if got.Freqs.Daily {
		t.Error("freq_daily aurait dû être forcé à 0")
	}
}
