package render

import (
	"strconv"
	"strings"

	"github.com/Sydnec/AnimeStats_DiscordBot/internal/stats"
)

// MessageLimit est la longueur maximale d'un message Discord.
//
// Le bot JS visait 4000 caractères, qui est la limite d'un embed (et le
// privilège Nitro côté client), pas celle du contenu d'un message : tout récap
// dépassant 2000 caractères était donc rejeté par l'API.
const MessageLimit = 2000

// DefaultMaxTitles est le nombre d'animes listés avant troncature.
const DefaultMaxTitles = 10

// EmptyTitles est le texte affiché quand aucun anime n'a été regardé.
const EmptyTitles = "Aucun anime enregistré"

// Report rassemble tout ce qu'il faut pour composer un récapitulatif.
type Report struct {
	// Header est la première ligne, par exemple « 📊 Stats AniList - septembre 2025 ».
	Header string

	TotalMinutes     float64
	AvgMinutesPerDay float64
	Episodes         int

	// TopDay est une journée « 2025-09-12 », vide s'il n'y en a pas.
	TopDay     string
	TopMinutes float64
	// ShowTopDay masque la ligne du jour le plus actif pour les périodes d'une
	// seule journée, où elle n'apporte rien.
	ShowTopDay bool

	Titles []stats.TitleCount

	// MaxTitles et Limit valent DefaultMaxTitles et MessageLimit si nuls.
	MaxTitles int
	Limit     int
}

// Message compose le récapitulatif, en réduisant la liste d'animes autant que
// nécessaire pour tenir dans la limite Discord.
func (r Report) Message() string {
	limit := r.Limit
	if limit <= 0 {
		limit = MessageLimit
	}
	maxTitles := r.MaxTitles
	if maxTitles <= 0 {
		maxTitles = DefaultMaxTitles
	}

	if len(r.Titles) == 0 {
		// Sans anime, la ligne du jour le plus actif n'a pas de sens.
		return truncateHard(r.head(false)+"\n"+EmptyTitles, limit)
	}

	for n := maxTitles; n >= 0; n-- {
		content := r.head(r.ShowTopDay) + "\n" + r.titleList(n)
		if length(content) <= limit {
			return content
		}
	}
	// Même sans aucun anime le message dépasse : l'en-tête est anormalement
	// long, on tronque proprement plutôt que de laisser l'API refuser l'envoi.
	return truncateHard(r.head(r.ShowTopDay), limit)
}

// head compose les lignes de synthèse.
func (r Report) head(withTopDay bool) string {
	var b strings.Builder
	b.WriteString(r.Header)
	b.WriteString("\n⏱️ Temps total : **")
	b.WriteString(Minutes(r.TotalMinutes))
	b.WriteString("**\n⏳ Moyenne journalière : **")
	b.WriteString(Minutes(r.AvgMinutesPerDay))
	b.WriteString("/j**\n🎬 Épisodes regardés : **")
	b.WriteString(strconv.Itoa(r.Episodes))
	b.WriteString("**")
	if withTopDay {
		b.WriteString("\n🔥 Jour le plus actif : **")
		b.WriteString(r.topDayText())
		b.WriteString("**")
	}
	b.WriteString("\n")
	return b.String()
}

func (r Report) topDayText() string {
	if r.TopDay == "" {
		return "N/A"
	}
	label, ok := DayKey(r.TopDay)
	if !ok {
		label = r.TopDay
	}
	return label + " (" + Minutes(r.TopMinutes) + ")"
}

// titleList compose la section des animes, limitée à n entrées.
func (r Report) titleList(n int) string {
	var b strings.Builder
	b.WriteString("# Animes :")
	for i, t := range r.Titles {
		if i >= n {
			break
		}
		b.WriteString("\n- ")
		b.WriteString(t.Title)
		b.WriteString(" (")
		b.WriteString(strconv.Itoa(t.Count))
		b.WriteString(")")
	}
	if len(r.Titles) > n {
		b.WriteString("\n...et ")
		b.WriteString(strconv.Itoa(len(r.Titles) - n))
		b.WriteString(" autres")
	}
	return b.String()
}

// length compte en unités UTF-16, comme le fait Discord — et comme le faisait
// la propriété .length de JavaScript. Compter les octets tronquerait bien trop
// tôt sur un texte français accentué et parsemé d'émojis.
func length(s string) int {
	n := 0
	for _, r := range s {
		n++
		if r > 0xFFFF {
			n++
		}
	}
	return n
}

// truncateHard coupe sur une frontière de caractère et signale la coupe.
func truncateHard(s string, limit int) string {
	if length(s) <= limit {
		return s
	}
	const ellipsis = "…"
	budget := limit - length(ellipsis)
	if budget <= 0 {
		return ""
	}

	n := 0
	for i, r := range s {
		w := 1
		if r > 0xFFFF {
			w = 2
		}
		if n+w > budget {
			return s[:i] + ellipsis
		}
		n += w
	}
	return s
}
