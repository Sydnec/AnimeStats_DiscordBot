package scheduler

import (
	"testing"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/Sydnec/AnimeStats_DiscordBot/internal/period"
)

func mustSchedule(t *testing.T, spec string) cron.Schedule {
	t.Helper()
	s, err := cron.ParseStandard(spec)
	if err != nil {
		t.Fatalf("expression cron %q invalide : %v", spec, err)
	}
	return s
}

func TestLastActivation(t *testing.T) {
	loc := time.UTC
	tests := []struct {
		name string
		spec string
		now  time.Time
		want string
	}{
		{"mensuel, milieu de mois", "0 10 1 * *",
			time.Date(2025, 9, 15, 0, 0, 0, 0, loc), "2025-09-01 10:00:00"},
		{"mensuel, avant l'heure du 1er", "0 10 1 * *",
			time.Date(2025, 9, 1, 9, 0, 0, 0, loc), "2025-08-01 10:00:00"},
		{"annuel, en mars", "0 12 1 1 *",
			time.Date(2025, 3, 1, 0, 0, 0, 0, loc), "2025-01-01 12:00:00"},
		{"annuel, avant midi le 1er janvier", "0 12 1 1 *",
			time.Date(2025, 1, 1, 11, 0, 0, 0, loc), "2024-01-01 12:00:00"},
		{"toutes les minutes", "* * * * *",
			time.Date(2025, 9, 15, 8, 30, 30, 0, loc), "2025-09-15 08:30:00"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := lastActivation(mustSchedule(t, tc.spec), tc.now)
			if !ok {
				t.Fatal("aucun déclenchement trouvé")
			}
			if s := got.Format("2006-01-02 15:04:05"); s != tc.want {
				t.Errorf("lastActivation = %s, attendu %s", s, tc.want)
			}
		})
	}
}

// TestCatchUpDecision reproduit le raisonnement de catchUpJob : le rattrapage
// n'a lieu que si l'heure d'envoi de la période courante est déjà passée.
func TestCatchUpDecision(t *testing.T) {
	loc := time.UTC
	schedule := mustSchedule(t, "0 10 1 * *")

	tests := []struct {
		name        string
		now         time.Time
		wantCatchUp bool
	}{
		{"le 1er à 9 h : l'envoi du mois n'a pas encore eu lieu", time.Date(2025, 9, 1, 9, 0, 0, 0, loc), false},
		{"le 1er à 11 h : l'envoi aurait dû partir", time.Date(2025, 9, 1, 11, 0, 0, 0, loc), true},
		{"le 15 : l'envoi aurait dû partir depuis deux semaines", time.Date(2025, 9, 15, 0, 0, 0, 0, loc), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := period.PreviousMonth(tc.now, loc)
			last, ok := lastActivation(schedule, tc.now)
			got := ok && period.PreviousMonth(last, loc).Key == p.Key
			if got != tc.wantCatchUp {
				t.Errorf("rattrapage = %v, attendu %v (période %s, dernier déclenchement %s)",
					got, tc.wantCatchUp, p.Key, last.Format(time.RFC3339))
			}
		})
	}
}
