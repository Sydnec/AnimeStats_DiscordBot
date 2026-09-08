package stats

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/Sydnec/AnimeStats_DiscordBot/internal/anilist"
)

// Les expressions rationnelles reprennent exactement celles de src/lib/stats.js.
//
// rangePattern n'est volontairement pas ancrée : c'était déjà le cas en JS, et
// certains libellés de progression d'AniList ne sont pas de simples plages.
var (
	rangePattern = regexp.MustCompile(`(\d+)\D+(\d+)`)
	intPattern   = regexp.MustCompile(`^\d+$`)
	// AniList écrit ses plages avec un tiret demi-cadratin ou cadratin selon
	// les cas ; on les ramène au tiret simple avant analyse.
	dashReplacer = strings.NewReplacer("–", "-", "—", "-")
)

// parsedProgress est le résultat de l'analyse du champ progress.
type parsedProgress struct {
	// Start et End délimitent la plage. Pour une valeur simple, les deux sont
	// égaux et IsRange vaut false.
	Start, End int
	IsRange    bool
	OK         bool
}

// parseProgress analyse « 3 », « 1-3 », « 1 - 3 », « 1 – 3 »…
//
// Attention : « 3.5 » est capté par la plage et interprété comme 3 → 5. Ce
// comportement est celui du bot JS, conservé pour ne pas modifier les
// statistiques d'un utilisateur existant.
func parseProgress(p anilist.Progress) parsedProgress {
	if !p.Valid {
		return parsedProgress{}
	}
	s := dashReplacer.Replace(strings.TrimSpace(p.Text))

	if m := rangePattern.FindStringSubmatch(s); m != nil {
		start, err1 := strconv.Atoi(m[1])
		end, err2 := strconv.Atoi(m[2])
		if err1 != nil || err2 != nil {
			return parsedProgress{}
		}
		return parsedProgress{Start: start, End: end, IsRange: true, OK: true}
	}
	if intPattern.MatchString(s) {
		v, err := strconv.Atoi(s)
		if err != nil {
			return parsedProgress{}
		}
		return parsedProgress{Start: v, End: v, OK: true}
	}
	return parsedProgress{}
}

// dedupKey reproduit la clé de déduplication du bot JS :
// `${media?.id || 'noid'}|${String(progress)}|${String(createdAt)}`.
func dedupKey(a anilist.Activity) string {
	progress := "null"
	if a.Progress.Valid {
		progress = a.Progress.Text
	}
	return a.Media.MediaID() + "|" + progress + "|" + strconv.FormatInt(a.CreatedAt, 10)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
