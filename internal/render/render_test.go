package render

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Sydnec/AnimeStats_DiscordBot/internal/stats"
)

func TestMinutes(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "0h"},
		{-1, "0h"},
		{59, "0h59min"},
		{60, "1h"},
		{372, "6h12min"},
		{12.4, "0h12min"},
		{6000, "100h"},
		{120.4, "2h"},
		{119.6, "2h"},
		{126, "2h6min"},
	}
	for _, tc := range tests {
		if got := Minutes(tc.in); got != tc.want {
			t.Errorf("Minutes(%v) = %q, attendu %q", tc.in, got, tc.want)
		}
	}
}

// TestDate vérifie la reproduction d'Intl.DateTimeFormat("fr-FR", {dateStyle:"medium"}).
func TestDate(t *testing.T) {
	tests := []struct {
		in   time.Time
		want string
	}{
		{time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), "1 janv. 2025"},
		{time.Date(2025, 2, 12, 0, 0, 0, 0, time.UTC), "12 févr. 2025"},
		{time.Date(2025, 3, 12, 0, 0, 0, 0, time.UTC), "12 mars 2025"},
		{time.Date(2025, 4, 12, 0, 0, 0, 0, time.UTC), "12 avr. 2025"},
		{time.Date(2025, 5, 12, 0, 0, 0, 0, time.UTC), "12 mai 2025"},
		{time.Date(2025, 6, 12, 0, 0, 0, 0, time.UTC), "12 juin 2025"},
		{time.Date(2025, 7, 9, 0, 0, 0, 0, time.UTC), "9 juil. 2025"},
		{time.Date(2025, 8, 12, 0, 0, 0, 0, time.UTC), "12 août 2025"},
		{time.Date(2025, 9, 12, 0, 0, 0, 0, time.UTC), "12 sept. 2025"},
		{time.Date(2025, 10, 12, 0, 0, 0, 0, time.UTC), "12 oct. 2025"},
		{time.Date(2025, 11, 12, 0, 0, 0, 0, time.UTC), "12 nov. 2025"},
		{time.Date(2025, 12, 12, 0, 0, 0, 0, time.UTC), "12 déc. 2025"},
	}
	for _, tc := range tests {
		if got := Date(tc.in); got != tc.want {
			t.Errorf("Date(%s) = %q, attendu %q", tc.in.Format("2006-01-02"), got, tc.want)
		}
	}
}

func TestMonthYear(t *testing.T) {
	tests := map[time.Month]string{
		time.January: "janvier 2025", time.February: "février 2025",
		time.March: "mars 2025", time.April: "avril 2025",
		time.May: "mai 2025", time.June: "juin 2025",
		time.July: "juillet 2025", time.August: "août 2025",
		time.September: "septembre 2025", time.October: "octobre 2025",
		time.November: "novembre 2025", time.December: "décembre 2025",
	}
	for m, want := range tests {
		got := MonthYear(time.Date(2025, m, 15, 0, 0, 0, 0, time.UTC))
		if got != want {
			t.Errorf("MonthYear(%s) = %q, attendu %q", m, got, want)
		}
	}
}

// Les chaînes attendues ci-dessous ont été produites par le bot JS d'origine :
// elles verrouillent la mise en forme au caractère près.
const (
	goldenComplet = "📊 Stats AniList - septembre 2025\n" +
		"⏱️ Temps total : **6h12min**\n" +
		"⏳ Moyenne journalière : **0h12min/j**\n" +
		"🎬 Épisodes regardés : **17**\n" +
		"🔥 Jour le plus actif : **12 sept. 2025 (2h6min)**\n" +
		"\n# Animes :\n- Frieren (9)\n- Dandadan (5)\n- Bocchi the Rock! (3)"

	goldenVide = "📆 Stats AniList - Année 2024\n" +
		"⏱️ Temps total : **0h**\n" +
		"⏳ Moyenne journalière : **0h/j**\n" +
		"🎬 Épisodes regardés : **0**\n" +
		"\nAucun anime enregistré"

	goldenSansJour = "📈 Récapitulatif - 8 sept. 2025 → 9 sept. 2025\n" +
		"⏱️ Temps total : **0h42min**\n" +
		"⏳ Moyenne journalière : **0h42min/j**\n" +
		"🎬 Épisodes regardés : **2**\n" +
		"\n# Animes :\n- Frieren (2)"
)

func TestMessageGolden(t *testing.T) {
	tests := []struct {
		name   string
		report Report
		want   string
	}{
		{
			name: "récapitulatif mensuel complet",
			report: Report{
				Header: "📊 Stats AniList - septembre 2025", TotalMinutes: 372,
				AvgMinutesPerDay: 12.4, Episodes: 17,
				TopDay: "2025-09-12", TopMinutes: 126, ShowTopDay: true,
				Titles: []stats.TitleCount{{Title: "Frieren", Count: 9},
					{Title: "Dandadan", Count: 5}, {Title: "Bocchi the Rock!", Count: 3}},
			},
			want: goldenComplet,
		},
		{
			name: "aucune activité",
			report: Report{
				Header: "📆 Stats AniList - Année 2024", ShowTopDay: true,
			},
			want: goldenVide,
		},
		{
			name: "période d'un seul jour, sans jour le plus actif",
			report: Report{
				Header:       "📈 Récapitulatif - 8 sept. 2025 → 9 sept. 2025",
				TotalMinutes: 42, AvgMinutesPerDay: 42, Episodes: 2,
				TopDay: "2025-09-08", TopMinutes: 42, ShowTopDay: false,
				Titles: []stats.TitleCount{{Title: "Frieren", Count: 2}},
			},
			want: goldenSansJour,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.report.Message(); got != tc.want {
				t.Errorf("message inattendu.\nobtenu :\n%s\n\nattendu :\n%s", got, tc.want)
			}
		})
	}
}

func TestMessageListsTenTitlesAndCountsTheRest(t *testing.T) {
	titles := make([]stats.TitleCount, 14)
	for i := range titles {
		titles[i] = stats.TitleCount{Title: fmt.Sprintf("Anime numéro %d", i+1), Count: 20 - i}
	}
	r := Report{
		Header: "📊 Stats AniList - septembre 2025", TotalMinutes: 6000,
		AvgMinutesPerDay: 200, Episodes: 250,
		TopDay: "2025-09-12", TopMinutes: 500, ShowTopDay: true, Titles: titles,
	}

	got := r.Message()
	if !strings.HasSuffix(got, "- Anime numéro 10 (11)\n...et 4 autres") {
		t.Errorf("liste tronquée inattendue :\n%s", got)
	}
	if strings.Contains(got, "Anime numéro 11") {
		t.Error("le onzième anime n'aurait pas dû être listé")
	}
}

func TestMessageNeverExceedsLimit(t *testing.T) {
	long := strings.Repeat("Titre très long ", 20)
	titles := make([]stats.TitleCount, 40)
	for i := range titles {
		titles[i] = stats.TitleCount{Title: fmt.Sprintf("%s %d 🎬", long, i), Count: 40 - i}
	}
	r := Report{
		Header: "📊 Stats AniList - septembre 2025", TotalMinutes: 6000,
		AvgMinutesPerDay: 200, Episodes: 250,
		TopDay: "2025-09-12", TopMinutes: 500, ShowTopDay: true, Titles: titles,
	}

	got := r.Message()
	if n := length(got); n > MessageLimit {
		t.Errorf("message de %d unités, la limite est %d", n, MessageLimit)
	}
	if !strings.Contains(got, "autres") {
		t.Errorf("le décompte des animes restants a disparu :\n%s", got)
	}
}

// TestMessageHardTruncation couvre le cas extrême où même l'en-tête dépasse.
func TestMessageHardTruncation(t *testing.T) {
	r := Report{
		Header: strings.Repeat("é🎬", 2000),
		Titles: []stats.TitleCount{{Title: "Frieren", Count: 1}},
	}
	got := r.Message()
	if n := length(got); n > MessageLimit {
		t.Errorf("message de %d unités, la limite est %d", n, MessageLimit)
	}
	if !strings.HasSuffix(got, "…") {
		t.Error("la troncature dure devrait être signalée par une ellipse")
	}
	// La coupe doit tomber sur une frontière de caractère.
	if !utf8Valid(got) {
		t.Error("la troncature a coupé au milieu d'un caractère")
	}
}

func utf8Valid(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}

func TestLengthCountsUTF16Units(t *testing.T) {
	// Un émoji hors du plan multilingue de base compte pour deux unités,
	// comme dans la propriété .length de JavaScript et pour Discord.
	if got := length("🎬"); got != 2 {
		t.Errorf("length(\"🎬\") = %d, attendu 2", got)
	}
	if got := length("é"); got != 1 {
		t.Errorf("length(\"é\") = %d, attendu 1", got)
	}
	if got := length("⏱️"); got != 2 {
		t.Errorf("length(\"⏱️\") = %d, attendu 2 (caractère + sélecteur de variante)", got)
	}
}

func TestDayKey(t *testing.T) {
	if got, ok := DayKey("2025-09-12"); !ok || got != "12 sept. 2025" {
		t.Errorf("DayKey = %q, %v", got, ok)
	}
	if _, ok := DayKey("pas une date"); ok {
		t.Error("DayKey aurait dû échouer")
	}
}

func TestTopDayFallback(t *testing.T) {
	r := Report{Header: "En-tête", ShowTopDay: true,
		Titles: []stats.TitleCount{{Title: "A", Count: 1}}}
	if !strings.Contains(r.Message(), "Jour le plus actif : **N/A**") {
		t.Errorf("sans jour le plus actif, N/A était attendu :\n%s", r.Message())
	}
}
