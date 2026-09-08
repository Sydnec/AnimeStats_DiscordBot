package stats

import (
	"testing"
	"time"

	"github.com/Sydnec/AnimeStats_DiscordBot/internal/anilist"
)

func ptr[T any](v T) *T { return &v }

func media(id int, title string, duration, episodes int) *anilist.Media {
	return &anilist.Media{
		ID:       id,
		Title:    anilist.Title{Romaji: title},
		Duration: ptr(duration),
		Episodes: ptr(episodes),
	}
}

// act construit une activité de progression.
func act(id int, at time.Time, progress string, m *anilist.Media) anilist.Activity {
	return anilist.Activity{
		ID:        id,
		CreatedAt: at.Unix(),
		Status:    "watched episode",
		Progress:  anilist.NewProgress(progress),
		Media:     m,
	}
}

// statusAct construit une activité de changement de statut, sans progression.
func statusAct(id int, at time.Time, status string, m *anilist.Media) anilist.Activity {
	return anilist.Activity{
		ID:        id,
		CreatedAt: at.Unix(),
		Status:    status,
		Media:     m,
	}
}

var (
	ref   = time.Date(2025, 9, 15, 12, 0, 0, 0, time.UTC)
	start = ref.AddDate(0, 0, -7)
	end   = ref
)

// TestPortedFromJavaScript reprend les trois scénarios des tests du bot Node.
//
// Les assertions d'origine attendaient 24 minutes par épisode alors que la
// soustraction de l'opening et de l'ending était déjà en place : elles
// échouaient. Chaque cas est donc décliné pour les deux réglages, ce qui
// verrouille séparément le comptage d'épisodes et la formule de durée.
func TestPortedFromJavaScript(t *testing.T) {
	nowSec := ref.Add(-1000 * time.Second)
	oneDay := 24 * time.Hour

	dup := media(10, "Dup", 24, 12)
	duplicateInput := Input{Activities: []anilist.Activity{
		// Deux activités strictement identiques : la déduplication ne doit en
		// retenir qu'une.
		{CreatedAt: nowSec.Unix(), Status: "CURRENT", Progress: anilist.NewProgress("3"), Media: dup},
		{CreatedAt: nowSec.Unix(), Status: "CURRENT", Progress: anilist.NewProgress("3"), Media: dup},
	}}

	rangeInput := Input{Activities: []anilist.Activity{
		act(1, nowSec, "1 - 3", media(20, "Range", 24, 0)),
	}}

	// Cliché MediaListCollection : anime A de 1 à 3 épisodes, anime B de 0 à 2.
	snapshotInput := Input{Lists: []anilist.MediaList{{Entries: []anilist.MediaListEntry{
		{Media: media(1, "A", 24, 0), Progress: anilist.NewProgress("1"), UpdatedAt: ref.Add(-2 * oneDay).Unix()},
		{Media: media(1, "A", 24, 0), Progress: anilist.NewProgress("3"), UpdatedAt: ref.Add(-oneDay).Unix()},
		{Media: media(2, "B", 24, 0), Progress: anilist.NewProgress("2"), UpdatedAt: ref.Add(-oneDay).Unix()},
	}}}}

	tests := []struct {
		name     string
		in       Input
		opEd     float64
		wantEps  int
		wantMins float64
	}{
		{"activités dupliquées, sans OP/ED", duplicateInput, 0, 3, 3 * 24},
		{"activités dupliquées, OP/ED de 3 min", duplicateInput, 3, 3, 3 * 21},
		{"plage 1 - 3, sans OP/ED", rangeInput, 0, 3, 3 * 24},
		{"plage 1 - 3, OP/ED de 3 min", rangeInput, 3, 3, 3 * 21},
		{"cliché de liste, sans OP/ED", snapshotInput, 0, 5, 5 * 24},
		{"cliché de liste, OP/ED de 3 min", snapshotInput, 3, 5, 5 * 21},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Compute(tc.in, start, end, Options{OpEdMinutes: tc.opEd})
			if got.TotalEpisodes != tc.wantEps {
				t.Errorf("TotalEpisodes = %d, attendu %d", got.TotalEpisodes, tc.wantEps)
			}
			if got.TotalMinutes != tc.wantMins {
				t.Errorf("TotalMinutes = %v, attendu %v", got.TotalMinutes, tc.wantMins)
			}
		})
	}
}

func TestDeltaTracking(t *testing.T) {
	m := media(1, "Delta", 24, 12)
	in := Input{Activities: []anilist.Activity{
		act(1, ref.Add(-72*time.Hour), "2", m),
		act(2, ref.Add(-48*time.Hour), "5", m),
		act(3, ref.Add(-24*time.Hour), "5", m), // aucune progression : 0 épisode
		act(4, ref.Add(-12*time.Hour), "8", m),
	}}

	got := Compute(in, start, end, Options{OpEdMinutes: 0})
	// 2 (premier, prev inconnu) + 3 + 0 + 3 = 8
	if got.TotalEpisodes != 8 {
		t.Errorf("TotalEpisodes = %d, attendu 8", got.TotalEpisodes)
	}
	if len(got.Titles) != 1 || got.Titles[0].Count != 8 {
		t.Errorf("Titles = %+v", got.Titles)
	}
}

func TestActivityOutsidePeriodStillSetsBaseline(t *testing.T) {
	m := media(1, "Base", 24, 12)
	in := Input{Activities: []anilist.Activity{
		// Bien avant la période : ne compte pas, mais fixe la progression.
		act(1, start.Add(-48*time.Hour), "5", m),
		act(2, ref.Add(-time.Hour), "8", m),
	}}

	got := Compute(in, start, end, Options{OpEdMinutes: 0})
	if got.TotalEpisodes != 3 {
		t.Errorf("TotalEpisodes = %d, attendu 3 (8 - 5)", got.TotalEpisodes)
	}
}

func TestRangeWithKnownPrevious(t *testing.T) {
	m := media(1, "Range", 24, 12)
	in := Input{Activities: []anilist.Activity{
		act(1, start.Add(-24*time.Hour), "4", m),
		// La plage 1-6 avec un antécédent à 4 ne vaut que deux épisodes.
		act(2, ref.Add(-time.Hour), "1 - 6", m),
	}}

	got := Compute(in, start, end, Options{OpEdMinutes: 0})
	if got.TotalEpisodes != 2 {
		t.Errorf("TotalEpisodes = %d, attendu 2", got.TotalEpisodes)
	}
}

func TestCompletedStatusCountsRemainingEpisodes(t *testing.T) {
	m := media(1, "Fini", 24, 12)
	in := Input{Activities: []anilist.Activity{
		act(1, ref.Add(-48*time.Hour), "10", m),
		statusAct(2, ref.Add(-time.Hour), "completed", m),
	}}

	got := Compute(in, start, end, Options{OpEdMinutes: 0})
	// 10 épisodes puis les 2 restants jusqu'au total de 12.
	if got.TotalEpisodes != 12 {
		t.Errorf("TotalEpisodes = %d, attendu 12", got.TotalEpisodes)
	}
}

func TestOtherStatusesCountNothing(t *testing.T) {
	m := media(1, "Abandonné", 24, 12)
	for _, status := range []string{"dropped", "paused", "planning", ""} {
		in := Input{Activities: []anilist.Activity{statusAct(1, ref.Add(-time.Hour), status, m)}}
		got := Compute(in, start, end, Options{OpEdMinutes: 0})
		if got.TotalEpisodes != 0 {
			t.Errorf("statut %q : TotalEpisodes = %d, attendu 0", status, got.TotalEpisodes)
		}
	}
}

func TestEpisodeDurationFallbacks(t *testing.T) {
	tests := []struct {
		name     string
		duration *int
		opEd     float64
		want     float64
	}{
		{"durée absente", nil, 0, 24},
		{"durée nulle traitée comme absente", ptr(0), 0, 24},
		{"durée renseignée", ptr(50), 3, 47},
		{"plancher d'une minute", ptr(2), 10, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &anilist.Media{ID: 1, Title: anilist.Title{Romaji: "T"}, Duration: tc.duration}
			in := Input{Activities: []anilist.Activity{act(1, ref.Add(-time.Hour), "1", m)}}
			got := Compute(in, start, end, Options{OpEdMinutes: tc.opEd})
			if got.TotalMinutes != tc.want {
				t.Errorf("TotalMinutes = %v, attendu %v", got.TotalMinutes, tc.want)
			}
		})
	}
}

func TestTitlePreferenceAndFallback(t *testing.T) {
	in := Input{Activities: []anilist.Activity{
		act(1, ref.Add(-time.Hour), "1", &anilist.Media{ID: 1, Title: anilist.Title{English: "Anglais"}}),
		act(2, ref.Add(-2*time.Hour), "1", &anilist.Media{ID: 2}),
	}}
	got := Compute(in, start, end, Options{OpEdMinutes: 0})

	titles := map[string]int{}
	for _, tc := range got.Titles {
		titles[tc.Title] = tc.Count
	}
	if titles["Anglais"] != 1 {
		t.Errorf("titre anglais absent : %+v", got.Titles)
	}
	if titles[anilist.TitreInconnu] != 1 {
		t.Errorf("titre de repli absent : %+v", got.Titles)
	}
}

func TestUnknownProgressFormatIsIgnored(t *testing.T) {
	m := media(1, "Bizarre", 24, 12)
	in := Input{Activities: []anilist.Activity{
		act(1, ref.Add(-time.Hour), "rewatched", m),
	}}
	got := Compute(in, start, end, Options{OpEdMinutes: 0})
	if got.TotalEpisodes != 0 {
		t.Errorf("TotalEpisodes = %d, attendu 0", got.TotalEpisodes)
	}
}

// TestTopDayTieBreak documente le départage : à minutes égales, la journée la
// plus ancienne l'emporte, comme dans le bot JS où la comparaison était
// strictement supérieure sur un objet parcouru dans l'ordre d'insertion.
func TestTopDayTieBreak(t *testing.T) {
	a := media(1, "A", 24, 12)
	b := media(2, "B", 24, 12)
	in := Input{Activities: []anilist.Activity{
		act(1, ref.Add(-72*time.Hour), "1", a),
		act(2, ref.Add(-24*time.Hour), "1", b),
	}}

	got := Compute(in, start, end, Options{OpEdMinutes: 0, Location: time.UTC})
	wantDay := ref.Add(-72 * time.Hour).UTC().Format("2006-01-02")
	if got.TopDay != wantDay {
		t.Errorf("TopDay = %q, attendu %q (la journée la plus ancienne)", got.TopDay, wantDay)
	}
	if got.TopMinutes != 24 {
		t.Errorf("TopMinutes = %v, attendu 24", got.TopMinutes)
	}
}

// TestTitlesStableOrder vérifie qu'à nombre d'épisodes égal, l'ordre
// d'apparition est conservé — condition d'une sortie reproductible.
func TestTitlesStableOrder(t *testing.T) {
	first := media(1, "Premier", 24, 12)
	second := media(2, "Second", 24, 12)
	third := media(3, "Troisième", 24, 12)

	in := Input{Activities: []anilist.Activity{
		act(1, ref.Add(-72*time.Hour), "1", first),
		act(2, ref.Add(-48*time.Hour), "1", second),
		act(3, ref.Add(-24*time.Hour), "5", third),
	}}

	for i := 0; i < 20; i++ {
		got := Compute(in, start, end, Options{OpEdMinutes: 0})
		want := []string{"Troisième", "Premier", "Second"}
		if len(got.Titles) != len(want) {
			t.Fatalf("Titles = %+v", got.Titles)
		}
		for j, w := range want {
			if got.Titles[j].Title != w {
				t.Fatalf("Titles = %+v, attendu %v", got.Titles, want)
			}
		}
	}
}

// TestDayBucketingUsesConfiguredLocation couvre la correction du découpage par
// jour : le bot JS regroupait en UTC alors que les périodes étaient calculées
// en heure locale.
func TestDayBucketingUsesConfiguredLocation(t *testing.T) {
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Skipf("fuseau Europe/Paris indisponible : %v", err)
	}
	// 1er septembre 2025 à 00 h 30 à Paris, soit le 31 août à 22 h 30 UTC.
	at := time.Date(2025, 9, 1, 0, 30, 0, 0, paris)
	in := Input{Activities: []anilist.Activity{act(1, at, "1", media(1, "Nuit", 24, 12))}}

	windowStart := at.Add(-time.Hour)
	windowEnd := at.Add(time.Hour)

	if got := Compute(in, windowStart, windowEnd, Options{Location: paris}); got.TopDay != "2025-09-01" {
		t.Errorf("à Paris, TopDay = %q, attendu 2025-09-01", got.TopDay)
	}
	if got := Compute(in, windowStart, windowEnd, Options{Location: time.UTC}); got.TopDay != "2025-08-31" {
		t.Errorf("en UTC, TopDay = %q, attendu 2025-08-31", got.TopDay)
	}
}

func TestEmptyInput(t *testing.T) {
	got := Compute(Input{}, start, end, Options{})
	if got.TotalEpisodes != 0 || got.TotalMinutes != 0 || got.TopDay != "" || len(got.Titles) != 0 {
		t.Errorf("résultat non vide : %+v", got)
	}
}

func TestActivitiesWithoutMediaShareFallbackKey(t *testing.T) {
	// Le bot JS regroupe sous la clé « noid » toute activité sans média.
	in := Input{Activities: []anilist.Activity{
		{ID: 1, CreatedAt: ref.Add(-2 * time.Hour).Unix(), Progress: anilist.NewProgress("2")},
		{ID: 2, CreatedAt: ref.Add(-time.Hour).Unix(), Progress: anilist.NewProgress("5")},
	}}
	got := Compute(in, start, end, Options{OpEdMinutes: 0})
	// 2 puis 3, avec la durée de repli de 24 minutes.
	if got.TotalEpisodes != 5 {
		t.Errorf("TotalEpisodes = %d, attendu 5", got.TotalEpisodes)
	}
	if got.TotalMinutes != 5*24 {
		t.Errorf("TotalMinutes = %v, attendu %v", got.TotalMinutes, 5*24)
	}
}

func TestPreviousProgressFromListSnapshot(t *testing.T) {
	m := media(1, "Repli", 24, 12)
	in := Input{
		Activities: []anilist.Activity{act(1, ref.Add(-time.Hour), "8", m)},
		Lists: []anilist.MediaList{{Entries: []anilist.MediaListEntry{
			// Cliché antérieur : la progression de départ est 5.
			{Media: m, Progress: anilist.NewProgress("5"), UpdatedAt: start.Add(-48 * time.Hour).Unix()},
		}}},
	}
	got := Compute(in, start, end, Options{OpEdMinutes: 0})
	if got.TotalEpisodes != 3 {
		t.Errorf("TotalEpisodes = %d, attendu 3 (8 - 5)", got.TotalEpisodes)
	}
}
