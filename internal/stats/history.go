package stats

import (
	"strconv"

	"github.com/Sydnec/AnimeStats_DiscordBot/internal/anilist"
)

// history retrouve la progression d'un média avant un instant donné.
//
// Le bot JS rebalayait l'intégralité du tableau d'activités à chaque appel, soit
// un coût quadratique sur mille activités. Les entrées sont ici indexées par
// média une fois pour toutes ; l'ordre d'origine est préservé, car c'est lui qui
// départage deux entrées de même horodatage.
type history struct {
	activities map[string][]anilist.Activity
	entries    map[string][]anilist.MediaListEntry
}

func newHistory(in Input) *history {
	h := &history{
		activities: make(map[string][]anilist.Activity),
		entries:    make(map[string][]anilist.MediaListEntry),
	}
	for _, a := range in.Activities {
		// Les activités sans média sont ignorées, comme en JS : elles ne
		// peuvent renseigner la progression d'aucun anime identifiable.
		if a.Media == nil {
			continue
		}
		id := a.Media.MediaID()
		h.activities[id] = append(h.activities[id], a)
	}
	for _, l := range in.Lists {
		for _, e := range l.Entries {
			if e.Media == nil {
				continue
			}
			id := strconv.Itoa(e.Media.ID)
			h.entries[id] = append(h.entries[id], e)
		}
	}
	return h
}

// previousProgress renvoie la dernière progression connue pour un média avant
// beforeSec, en consultant d'abord les activités puis, à défaut, les entrées de
// liste. Le booléen indique si une valeur a été trouvée.
func (h *history) previousProgress(mediaID string, beforeSec int64) (int, bool) {
	var (
		bestVal int
		bestTS  int64
		found   bool
	)

	for _, a := range h.activities[mediaID] {
		if a.CreatedAt == 0 || a.CreatedAt >= beforeSec {
			continue
		}
		parsed := parseProgress(a.Progress)
		if !parsed.OK {
			continue
		}
		// Comparaison stricte : à horodatage égal, la première entrée
		// rencontrée dans l'ordre d'origine l'emporte.
		if !found || a.CreatedAt > bestTS {
			bestVal, bestTS, found = parsed.End, a.CreatedAt, true
		}
	}
	if found {
		return bestVal, true
	}

	for _, e := range h.entries[mediaID] {
		if e.UpdatedAt == 0 || e.UpdatedAt >= beforeSec {
			continue
		}
		v, ok := snapshotProgress(e.Progress)
		if !ok {
			continue
		}
		if !found || e.UpdatedAt > bestTS {
			bestVal, bestTS, found = v, e.UpdatedAt, true
		}
	}
	return bestVal, found
}

// snapshotProgress lit la progression d'une entrée de liste, qui est un entier
// et non une plage d'épisodes.
func snapshotProgress(p anilist.Progress) (int, bool) {
	if !p.Valid {
		return 0, false
	}
	v, err := strconv.Atoi(p.Text)
	if err != nil {
		return 0, false
	}
	return v, true
}
