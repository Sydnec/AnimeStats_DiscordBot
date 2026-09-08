package report

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sydnec/AnimeStats_DiscordBot/internal/anilist"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/mask"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/period"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/stats"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/store"
)

// fakeDM enregistre les messages au lieu de les envoyer.
type fakeDM struct {
	sent map[string][]string
	fail map[string]error
}

func newFakeDM() *fakeDM {
	return &fakeDM{sent: map[string][]string{}, fail: map[string]error{}}
}

func (f *fakeDM) SendDM(_ context.Context, userID, content string) error {
	if err, ok := f.fail[userID]; ok {
		return err
	}
	f.sent[userID] = append(f.sent[userID], content)
	return nil
}

// anilistStub sert un flux d'activités figé.
func anilistStub(t *testing.T, activities string) *anilist.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("lecture du corps : %v", err)
			return
		}
		if strings.Contains(string(body), "User(name") {
			fmt.Fprint(w, `{"data":{"User":{"id":1}}}`)
			return
		}
		fmt.Fprintf(w, `{"data":{"Page":{"pageInfo":{"hasNextPage":false},"activities":[%s]}}}`, activities)
	}))
	t.Cleanup(srv.Close)

	return anilist.New(anilist.Options{Endpoint: srv.URL, RatePerMinute: -1})
}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("ouverture de la base : %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func activity(id int, at time.Time, progress string) string {
	return fmt.Sprintf(
		`{"id":%d,"createdAt":%d,"status":"watched episode","progress":%q,`+
			`"media":{"id":7,"status":"FINISHED","episodes":12,"duration":24,`+
			`"title":{"romaji":"Frieren","english":null,"native":null}}}`,
		id, at.Unix(), progress)
}

// TestSendProducesCompleteRecap parcourt toute la chaîne : récupération des
// activités, calcul, mise en forme et envoi.
func TestSendProducesCompleteRecap(t *testing.T) {
	loc := time.UTC
	now := time.Date(2025, 9, 20, 12, 0, 0, 0, loc)
	p := period.LastDays(now, loc, 7)

	acts := strings.Join([]string{
		activity(1, now.Add(-48*time.Hour), "3"),
		activity(2, now.Add(-24*time.Hour), "6"),
	}, ",")

	dm := newFakeDM()
	svc := &Service{
		AniList: anilistStub(t, acts),
		Store:   newStore(t),
		DM:      dm,
		Log:     slog.New(slog.DiscardHandler),
		Stats:   stats.Options{OpEdMinutes: 3, Location: loc},
	}

	if err := svc.Send(context.Background(), "user-1", "Sydnec", p); err != nil {
		t.Fatalf("Send : %v", err)
	}

	msgs := dm.sent["user-1"]
	if len(msgs) != 1 {
		t.Fatalf("%d message(s) envoyé(s), attendu 1", len(msgs))
	}
	got := msgs[0]

	// 3 puis 3 épisodes, à 21 minutes l'unité, soit 126 minutes.
	for _, want := range []string{
		"📈 Récapitulatif - 13 sept. 2025 → 20 sept. 2025",
		"⏱️ Temps total : **2h6min**",
		"🎬 Épisodes regardés : **6**",
		"- Frieren (6)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("le message ne contient pas %q :\n%s", want, got)
		}
	}
	// 126 minutes sur 7 jours, soit 18 minutes par jour.
	if !strings.Contains(got, "⏳ Moyenne journalière : **0h18min/j**") {
		t.Errorf("moyenne journalière inattendue :\n%s", got)
	}
}

func TestSendRefusesEmptyUsername(t *testing.T) {
	svc := &Service{Log: slog.New(slog.DiscardHandler)}
	err := svc.Send(context.Background(), "user-1", "", period.LastDays(time.Now(), time.UTC, 7))
	if err == nil {
		t.Fatal("une erreur était attendue")
	}
}

// TestBroadcastIsolatesFailures vérifie qu'un abonné en échec n'empêche pas
// les autres de recevoir leur récapitulatif.
func TestBroadcastIsolatesFailures(t *testing.T) {
	ctx := context.Background()
	loc := time.UTC
	now := time.Date(2025, 9, 20, 12, 0, 0, 0, loc)

	st := newStore(t)
	for _, u := range []string{"a", "b", "c"} {
		if err := st.Upsert(ctx, u, "pseudo-"+u, mask.Freqs{Monthly: true}); err != nil {
			t.Fatalf("Upsert(%s) : %v", u, err)
		}
	}
	// Un abonné annuel ne doit pas être concerné par la diffusion mensuelle.
	if err := st.Upsert(ctx, "d", "pseudo-d", mask.Freqs{Yearly: true}); err != nil {
		t.Fatalf("Upsert(d) : %v", err)
	}

	dm := newFakeDM()
	dm.fail["b"] = errors.New("messages privés fermés")

	svc := &Service{
		AniList: anilistStub(t, activity(1, now.Add(-40*24*time.Hour), "3")),
		Store:   st,
		DM:      dm,
		Log:     slog.New(slog.DiscardHandler),
		Stats:   stats.Options{OpEdMinutes: 3, Location: loc},
	}

	sent, failed := svc.Broadcast(ctx, store.Monthly, period.PreviousMonth(now, loc))
	if sent != 2 || failed != 1 {
		t.Errorf("Broadcast = %d envoyés / %d échecs, attendu 2 / 1", sent, failed)
	}
	if len(dm.sent["a"]) != 1 || len(dm.sent["c"]) != 1 {
		t.Errorf("les abonnés a et c auraient dû recevoir un message : %+v", dm.sent)
	}
	if _, ok := dm.sent["d"]; ok {
		t.Error("l'abonné annuel n'aurait pas dû recevoir la diffusion mensuelle")
	}
}

func TestBroadcastWithoutFollowers(t *testing.T) {
	svc := &Service{
		Store: newStore(t),
		DM:    newFakeDM(),
		Log:   slog.New(slog.DiscardHandler),
	}
	sent, failed := svc.Broadcast(context.Background(), store.Monthly,
		period.PreviousMonth(time.Now(), time.UTC))
	if sent != 0 || failed != 0 {
		t.Errorf("Broadcast = %d / %d, attendu 0 / 0", sent, failed)
	}
}

func TestBroadcastStopsOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	st := newStore(t)
	for _, u := range []string{"a", "b"} {
		if err := st.Upsert(context.Background(), u, "pseudo", mask.Freqs{Monthly: true}); err != nil {
			t.Fatalf("Upsert : %v", err)
		}
	}
	cancel()

	svc := &Service{
		AniList: anilistStub(t, ""),
		Store:   st,
		DM:      newFakeDM(),
		Log:     slog.New(slog.DiscardHandler),
	}
	sent, failed := svc.Broadcast(ctx, store.Monthly, period.PreviousMonth(time.Now(), time.UTC))
	if sent != 0 || failed != 0 {
		t.Errorf("Broadcast = %d / %d, attendu 0 / 0 sur contexte annulé", sent, failed)
	}
}
