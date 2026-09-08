// Package stats calcule le temps de visionnage et le nombre d'épisodes à
// partir des données AniList.
//
// C'est le portage fidèle de src/lib/stats.js. Le paquet est volontairement
// pur : ni réseau, ni base de données, ni Discord. Il prend des structures en
// entrée et rend un résultat, ce qui le rend testable exhaustivement.
package stats

import (
	"sort"
	"time"

	"github.com/Sydnec/AnimeStats_DiscordBot/internal/anilist"
)

// DefaultOpEdMinutes est la durée retirée par épisode pour compenser
// l'opening et l'ending.
const DefaultOpEdMinutes = 3.0

// Options paramètre le calcul.
type Options struct {
	// OpEdMinutes est retiré de la durée de chaque épisode, avec un plancher
	// d'une minute.
	OpEdMinutes float64
	// Location est le fuseau dans lequel les journées sont découpées.
	//
	// Le bot JS regroupait par jour en UTC alors que les bornes de période
	// étaient calculées en heure locale : un épisode regardé le 1er du mois à
	// 1 h du matin était compté dans le mois mais attribué au dernier jour du
	// mois précédent. Passer le fuseau explicitement corrige l'incohérence.
	Location *time.Location
}

func (o Options) location() *time.Location {
	if o.Location == nil {
		return time.UTC
	}
	return o.Location
}

// TitleCount est le nombre d'épisodes regardés pour un anime.
type TitleCount struct {
	Title string
	Count int
}

// Result est le bilan d'une période.
type Result struct {
	TotalMinutes  float64
	TotalEpisodes int
	// Daily associe une journée ("2006-01-02") au nombre de minutes.
	Daily map[string]float64
	// Days liste les journées dans leur ordre d'apparition, ce qui rend le
	// départage du jour le plus actif déterministe.
	Days []string
	// TopDay est la journée la plus chargée, vide si la période est vide.
	TopDay     string
	TopMinutes float64
	// Titles est trié par nombre d'épisodes décroissant, les ex æquo restant
	// dans leur ordre d'apparition.
	Titles []TitleCount
}

// Input regroupe les données brutes exploitables.
type Input struct {
	Activities []anilist.Activity
	// Lists n'est plus alimenté par le client, mais le calcul sait toujours
	// l'exploiter : c'est le format des jeux d'essai historiques, et il sert
	// de repli pour retrouver une progression antérieure.
	Lists []anilist.MediaList
}

// accumulator agrège les compteurs au fil du parcours.
type accumulator struct {
	totalMinutes  float64
	totalEpisodes int
	daily         map[string]float64
	days          []string
	titleCounts   map[string]int
	titleOrder    []string
	loc           *time.Location
}

func newAccumulator(loc *time.Location) *accumulator {
	return &accumulator{
		daily:       make(map[string]float64),
		titleCounts: make(map[string]int),
		loc:         loc,
	}
}

func (a *accumulator) add(date time.Time, title string, episodes int, minutes float64) {
	a.totalMinutes += minutes
	a.totalEpisodes += episodes

	day := date.In(a.loc).Format("2006-01-02")
	if _, seen := a.daily[day]; !seen {
		a.days = append(a.days, day)
	}
	a.daily[day] += minutes

	if _, seen := a.titleCounts[title]; !seen {
		a.titleOrder = append(a.titleOrder, title)
	}
	a.titleCounts[title] += episodes
}

func (a *accumulator) result() Result {
	r := Result{
		TotalMinutes:  a.totalMinutes,
		TotalEpisodes: a.totalEpisodes,
		Daily:         a.daily,
		Days:          a.days,
	}

	// Comparaison strictement supérieure sur l'ordre d'apparition : en cas
	// d'égalité, la première journée rencontrée l'emporte, comme en JS où
	// l'itération d'un objet suit l'ordre d'insertion.
	for _, day := range a.days {
		if m := a.daily[day]; m > r.TopMinutes {
			r.TopMinutes = m
			r.TopDay = day
		}
	}

	r.Titles = make([]TitleCount, 0, len(a.titleOrder))
	for _, t := range a.titleOrder {
		r.Titles = append(r.Titles, TitleCount{Title: t, Count: a.titleCounts[t]})
	}
	// Tri stable : à nombre d'épisodes égal, l'ordre d'apparition est conservé.
	sort.SliceStable(r.Titles, func(i, j int) bool { return r.Titles[i].Count > r.Titles[j].Count })

	return r
}

// episodeMinutes calcule la durée retenue pour un épisode du média donné.
func episodeMinutes(m *anilist.Media, opEd float64) float64 {
	d := float64(m.EpisodeDuration()) - opEd
	if d < 1 {
		return 1
	}
	return d
}

// Compute calcule les statistiques sur la période [start, end].
//
// Comme dans le bot d'origine, les activités priment : la liste de médias n'est
// exploitée comme source principale que si aucune activité n'est fournie.
func Compute(in Input, start, end time.Time, opts Options) Result {
	if len(in.Activities) > 0 {
		return computeFromActivities(in, start, end, opts)
	}
	return computeFromLists(in.Lists, start, end, opts)
}

func computeFromActivities(in Input, start, end time.Time, opts Options) Result {
	acc := newAccumulator(opts.location())
	hist := newHistory(in)

	// Tri chronologique croissant, stable, pour suivre la progression de chaque
	// média dans l'ordre où elle a eu lieu.
	sorted := make([]anilist.Activity, len(in.Activities))
	copy(sorted, in.Activities)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].CreatedAt < sorted[j].CreatedAt })

	seen := make(map[string]struct{}, len(sorted))
	// lastProgress mémorise la dernière progression connue par média.
	lastProgress := make(map[string]int)

	for _, activity := range sorted {
		key := dedupKey(activity)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}

		if activity.CreatedAt == 0 {
			continue
		}
		date := time.Unix(activity.CreatedAt, 0)

		mediaID := activity.Media.MediaID()
		prev, hasPrev := lastProgress[mediaID]

		episodes := 0
		parsed := parseProgress(activity.Progress)

		switch {
		case parsed.OK && parsed.IsRange:
			if !hasPrev {
				if found, ok := hist.previousProgress(mediaID, activity.CreatedAt); ok {
					prev, hasPrev = found, true
					episodes = maxInt(0, parsed.End-prev)
				} else {
					// Aucun antécédent : la plage « 9-10 » vaut deux épisodes,
					// et non dix comme le donnerait une lecture 0 → 10.
					episodes = maxInt(0, parsed.End-parsed.Start+1)
				}
			} else {
				episodes = maxInt(0, parsed.End-prev)
			}
			lastProgress[mediaID] = maxInt(prevOrZero(prev, hasPrev), parsed.End)

		case parsed.OK:
			if !hasPrev {
				if found, ok := hist.previousProgress(mediaID, activity.CreatedAt); ok {
					prev = found
				} else {
					prev = 0
				}
				hasPrev = true
			}
			episodes = maxInt(0, parsed.End-prev)
			lastProgress[mediaID] = maxInt(prev, parsed.End)

		case !activity.Progress.Valid:
			// Progression absente : c'est un changement de statut dans la liste
			// de l'utilisateur. Seul le passage à COMPLETED permet d'en déduire
			// des épisodes ; DROPPED, PAUSED ou PLANNING n'en comptent aucun.
			//
			// On lit bien activity.status, le statut dans la liste, et non
			// media.status qui décrit la diffusion de l'anime.
			if !isCompleted(activity.Status) {
				continue
			}
			total := activity.Media.EpisodeCount()
			if !hasPrev {
				if found, ok := hist.previousProgress(mediaID, activity.CreatedAt); ok {
					prev = found
				} else {
					prev = 0
				}
				hasPrev = true
			}
			episodes = maxInt(0, total-prev)
			lastProgress[mediaID] = maxInt(prev, total)

		default:
			// Format de progression non reconnu : rien n'est compté.
			continue
		}

		// Seules les activités de la période alimentent les compteurs, mais
		// toutes ont mis à jour la progression courante.
		if episodes > 0 && !date.Before(start) && !date.After(end) {
			acc.add(date, activity.Media.PreferredTitle(), episodes,
				float64(episodes)*episodeMinutes(activity.Media, opts.OpEdMinutes))
		}
	}

	return acc.result()
}

func prevOrZero(prev int, hasPrev bool) int {
	if !hasPrev {
		return 0
	}
	return prev
}

func isCompleted(status string) bool {
	// AniList écrit « completed » en minuscules dans les activités de liste.
	const completed = "COMPLETED"
	if len(status) != len(completed) {
		return false
	}
	for i := 0; i < len(status); i++ {
		c := status[i]
		if 'a' <= c && c <= 'z' {
			c -= 'a' - 'A'
		}
		if c != completed[i] {
			return false
		}
	}
	return true
}
