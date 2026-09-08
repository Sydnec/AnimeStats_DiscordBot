// Package period calcule les bornes des périodes couvertes par un récapitulatif.
package period

import (
	"fmt"
	"time"

	"github.com/Sydnec/AnimeStats_DiscordBot/internal/render"
)

// Kind identifie le type de période.
type Kind string

const (
	// Month couvre le mois calendaire précédent.
	Month Kind = "month"
	// Year couvre l'année précédente.
	Year Kind = "year"
	// Rolling couvre les N derniers jours.
	Rolling Kind = "rolling"
	// Day couvre la journée de la veille.
	Day Kind = "day"
)

// Period décrit une fenêtre temporelle et son en-tête.
type Period struct {
	Kind       Kind
	Start, End time.Time
	// Days est le nombre de jours de la période, utilisé pour la moyenne
	// journalière. Il est déduit du calendrier et non d'une durée en heures :
	// un mois de changement d'heure compte bien 31 jours et non 32.
	Days int
	// Header est la première ligne du message.
	Header string
	// ShowTopDay masque la ligne du jour le plus actif sur une période d'un
	// seul jour, où elle ferait doublon.
	ShowTopDay bool
	// Key identifie la période pour le suivi des envois ("2025-09", "2024").
	Key string
}

// endOfDay renvoie le dernier instant d'une journée.
func endOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), t.Location())
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// PreviousMonth couvre le mois calendaire précédant `now`.
func PreviousMonth(now time.Time, loc *time.Location) Period {
	n := now.In(loc)
	start := time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, loc).AddDate(0, -1, 0)
	end := endOfDay(start.AddDate(0, 1, -1))
	return Period{
		Kind:       Month,
		Start:      start,
		End:        end,
		Days:       daysBetween(start, end),
		Header:     "📊 Stats AniList - " + render.MonthYear(start),
		ShowTopDay: true,
		Key:        start.Format("2006-01"),
	}
}

// PreviousYear couvre l'année précédant `now`.
func PreviousYear(now time.Time, loc *time.Location) Period {
	year := now.In(loc).Year() - 1
	start := time.Date(year, time.January, 1, 0, 0, 0, 0, loc)
	end := endOfDay(time.Date(year, time.December, 31, 0, 0, 0, 0, loc))
	return Period{
		Kind:       Year,
		Start:      start,
		End:        end,
		Days:       daysBetween(start, end),
		Header:     fmt.Sprintf("📆 Stats AniList - Année %d", year),
		ShowTopDay: true,
		Key:        fmt.Sprintf("%d", year),
	}
}

// LastDays couvre les `days` derniers jours, jusqu'à maintenant.
func LastDays(now time.Time, loc *time.Location, days int) Period {
	if days < 1 {
		days = 1
	}
	end := now.In(loc)
	start := end.AddDate(0, 0, -days)
	return Period{
		Kind:  Rolling,
		Start: start,
		End:   end,
		Days:  days,
		Header: "📈 Récapitulatif - " + render.Date(start) +
			" → " + render.Date(end),
		// Sur une seule journée, le « jour le plus actif » est la journée
		// elle-même : la ligne n'apprend rien.
		ShowTopDay: days > 1,
		Key:        start.Format("2006-01-02") + "/" + end.Format("2006-01-02"),
	}
}

// PreviousDay couvre la journée de la veille.
//
// Le récapitulatif quotidien n'est plus planifié, mais la période reste
// disponible pour un envoi manuel.
func PreviousDay(now time.Time, loc *time.Location) Period {
	yesterday := now.In(loc).AddDate(0, 0, -1)
	start := startOfDay(yesterday)
	end := endOfDay(yesterday)
	return Period{
		Kind:       Day,
		Start:      start,
		End:        end,
		Days:       1,
		Header:     "📅 Récap journalier - " + render.Date(start),
		ShowTopDay: false,
		Key:        start.Format("2006-01-02"),
	}
}

// daysBetween compte les journées civiles couvertes, bornes incluses.
//
// Le calcul passe par le calendrier plutôt que par un écart en heures : le bot
// JS divisait une durée par 24 h, ce qui donnait 32 jours pour le mois
// d'octobre à cause du passage à l'heure d'hiver, et faussait donc la moyenne.
func daysBetween(start, end time.Time) int {
	s := startOfDay(start)
	e := startOfDay(end)
	days := 0
	for !s.After(e) {
		days++
		s = s.AddDate(0, 0, 1)
	}
	if days < 1 {
		return 1
	}
	return days
}
