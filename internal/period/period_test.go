package period

import (
	"testing"
	"time"
)

func paris(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Skipf("fuseau Europe/Paris indisponible : %v", err)
	}
	return loc
}

func TestPreviousMonth(t *testing.T) {
	loc := paris(t)
	tests := []struct {
		name       string
		now        time.Time
		wantStart  string
		wantEnd    string
		wantDays   int
		wantHeader string
		wantKey    string
	}{
		{
			name: "mois courant", now: time.Date(2025, 9, 15, 12, 0, 0, 0, loc),
			wantStart: "2025-08-01", wantEnd: "2025-08-31", wantDays: 31,
			wantHeader: "📊 Stats AniList - août 2025", wantKey: "2025-08",
		},
		{
			name: "passage d'année", now: time.Date(2025, 1, 1, 10, 0, 0, 0, loc),
			wantStart: "2024-12-01", wantEnd: "2024-12-31", wantDays: 31,
			wantHeader: "📊 Stats AniList - décembre 2024", wantKey: "2024-12",
		},
		{
			name: "février bissextile", now: time.Date(2024, 3, 1, 10, 0, 0, 0, loc),
			wantStart: "2024-02-01", wantEnd: "2024-02-29", wantDays: 29,
			wantHeader: "📊 Stats AniList - février 2024", wantKey: "2024-02",
		},
		{
			// Octobre contient le passage à l'heure d'hiver. Le bot JS divisait
			// une durée par 24 h et comptait donc 32 jours, ce qui faussait la
			// moyenne journalière.
			name: "mois du changement d'heure", now: time.Date(2025, 11, 5, 10, 0, 0, 0, loc),
			wantStart: "2025-10-01", wantEnd: "2025-10-31", wantDays: 31,
			wantHeader: "📊 Stats AniList - octobre 2025", wantKey: "2025-10",
		},
		{
			name: "mois du passage à l'heure d'été", now: time.Date(2025, 4, 2, 10, 0, 0, 0, loc),
			wantStart: "2025-03-01", wantEnd: "2025-03-31", wantDays: 31,
			wantHeader: "📊 Stats AniList - mars 2025", wantKey: "2025-03",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := PreviousMonth(tc.now, loc)
			if got := p.Start.Format("2006-01-02"); got != tc.wantStart {
				t.Errorf("Start = %s, attendu %s", got, tc.wantStart)
			}
			if got := p.End.Format("2006-01-02"); got != tc.wantEnd {
				t.Errorf("End = %s, attendu %s", got, tc.wantEnd)
			}
			if p.Start.Hour() != 0 || p.Start.Minute() != 0 {
				t.Errorf("Start devrait être à minuit : %s", p.Start)
			}
			if p.End.Hour() != 23 || p.End.Minute() != 59 || p.End.Second() != 59 {
				t.Errorf("End devrait être en fin de journée : %s", p.End)
			}
			if p.Days != tc.wantDays {
				t.Errorf("Days = %d, attendu %d", p.Days, tc.wantDays)
			}
			if p.Header != tc.wantHeader {
				t.Errorf("Header = %q, attendu %q", p.Header, tc.wantHeader)
			}
			if p.Key != tc.wantKey {
				t.Errorf("Key = %q, attendu %q", p.Key, tc.wantKey)
			}
			if !p.ShowTopDay {
				t.Error("le jour le plus actif devrait être affiché")
			}
		})
	}
}

func TestPreviousYear(t *testing.T) {
	loc := paris(t)
	p := PreviousYear(time.Date(2025, 6, 1, 0, 0, 0, 0, loc), loc)

	if got := p.Start.Format("2006-01-02"); got != "2024-01-01" {
		t.Errorf("Start = %s", got)
	}
	if got := p.End.Format("2006-01-02"); got != "2024-12-31" {
		t.Errorf("End = %s", got)
	}
	if p.Days != 366 {
		t.Errorf("Days = %d, attendu 366 (2024 est bissextile)", p.Days)
	}
	if p.Header != "📆 Stats AniList - Année 2024" {
		t.Errorf("Header = %q", p.Header)
	}
	if p.Key != "2024" {
		t.Errorf("Key = %q", p.Key)
	}

	nonLeap := PreviousYear(time.Date(2024, 6, 1, 0, 0, 0, 0, loc), loc)
	if nonLeap.Days != 365 {
		t.Errorf("Days = %d, attendu 365", nonLeap.Days)
	}
}

func TestLastDays(t *testing.T) {
	loc := paris(t)
	now := time.Date(2025, 9, 9, 18, 30, 0, 0, loc)

	p := LastDays(now, loc, 30)
	if p.Days != 30 {
		t.Errorf("Days = %d, attendu 30", p.Days)
	}
	if !p.ShowTopDay {
		t.Error("sur 30 jours, le jour le plus actif devrait être affiché")
	}
	if got := p.Start.Format("2006-01-02"); got != "2025-08-10" {
		t.Errorf("Start = %s, attendu 2025-08-10", got)
	}

	// Sur une seule journée, la ligne du jour le plus actif est masquée.
	one := LastDays(now, loc, 1)
	if one.ShowTopDay {
		t.Error("sur un jour, le jour le plus actif ne devrait pas être affiché")
	}
	if one.Header != "📈 Récapitulatif - 8 sept. 2025 → 9 sept. 2025" {
		t.Errorf("Header = %q", one.Header)
	}

	// Une valeur absurde est ramenée à un jour plutôt que de produire une
	// période inversée.
	if got := LastDays(now, loc, 0); got.Days != 1 {
		t.Errorf("Days = %d pour 0 jour, attendu 1", got.Days)
	}
}

func TestPreviousDay(t *testing.T) {
	loc := paris(t)
	p := PreviousDay(time.Date(2025, 9, 9, 3, 0, 0, 0, loc), loc)

	if got := p.Start.Format("2006-01-02 15:04:05"); got != "2025-09-08 00:00:00" {
		t.Errorf("Start = %s", got)
	}
	if got := p.End.Format("2006-01-02 15:04:05"); got != "2025-09-08 23:59:59" {
		t.Errorf("End = %s", got)
	}
	if p.Days != 1 || p.ShowTopDay {
		t.Errorf("Days = %d, ShowTopDay = %v", p.Days, p.ShowTopDay)
	}
	if p.Header != "📅 Récap journalier - 8 sept. 2025" {
		t.Errorf("Header = %q", p.Header)
	}
}
