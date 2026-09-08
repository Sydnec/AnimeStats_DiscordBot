package stats

import (
	"sort"
	"strconv"
	"time"

	"github.com/Sydnec/AnimeStats_DiscordBot/internal/anilist"
)

// computeFromLists calcule les statistiques à partir de clichés
// MediaListCollection, quand aucune activité n'est disponible.
//
// Le client n'interroge plus ce format, mais le calcul est conservé : c'est
// celui que couvrent les jeux d'essai hérités, et il reste le seul recours si
// le flux d'activités d'un utilisateur est vide.
func computeFromLists(lists []anilist.MediaList, start, end time.Time, opts Options) Result {
	acc := newAccumulator(opts.location())

	// Regroupement par média, en conservant l'ordre de première apparition.
	byMedia := make(map[string][]anilist.MediaListEntry)
	var order []string
	for _, l := range lists {
		for _, e := range l.Entries {
			if e.Media == nil {
				continue
			}
			id := strconv.Itoa(e.Media.ID)
			if _, seen := byMedia[id]; !seen {
				order = append(order, id)
			}
			byMedia[id] = append(byMedia[id], e)
		}
	}

	for _, id := range order {
		entries := byMedia[id]
		sort.SliceStable(entries, func(i, j int) bool { return entries[i].UpdatedAt < entries[j].UpdatedAt })

		var (
			prev        int
			hasPrev     bool
			title       string
			hasTitle    bool
			durationPer = anilist.DefaultEpisodeDuration
		)

		for _, entry := range entries {
			if entry.UpdatedAt == 0 {
				continue
			}
			date := time.Unix(entry.UpdatedAt, 0)

			if entry.Media != nil && entry.Media.Duration != nil && *entry.Media.Duration > 0 {
				durationPer = *entry.Media.Duration
			}

			// Une entrée hors période ne compte pas, mais elle met à jour la
			// progression de référence et les métadonnées du média.
			if date.Before(start) || date.After(end) {
				if v, ok := snapshotProgress(entry.Progress); ok {
					prev, hasPrev = v, true
				}
				if !hasTitle {
					if t := entry.Media.PreferredTitle(); t != anilist.TitreInconnu {
						title, hasTitle = t, true
					}
				}
				continue
			}

			progress, _ := snapshotProgress(entry.Progress)
			if !hasTitle {
				title, hasTitle = entry.Media.PreferredTitle(), true
			}

			episodes := progress
			if hasPrev {
				episodes = maxInt(0, progress-prev)
			}

			if episodes > 0 {
				minutes := float64(episodes) * episodeMinutesFor(durationPer, opts.OpEdMinutes)
				acc.add(date, title, episodes, minutes)
			}

			prev, hasPrev = progress, true
		}
	}

	return acc.result()
}

func episodeMinutesFor(duration int, opEd float64) float64 {
	d := float64(duration) - opEd
	if d < 1 {
		return 1
	}
	return d
}
