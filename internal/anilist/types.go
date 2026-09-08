// Package anilist interroge l'API GraphQL publique d'AniList.
package anilist

import (
	"bytes"
	"encoding/json"
	"strconv"
)

// TitreInconnu est le libellé de repli quand un média n'expose aucun titre.
const TitreInconnu = "Titre inconnu"

// DefaultEpisodeDuration est la durée retenue quand AniList ne renseigne pas
// media.duration, comme dans le bot d'origine.
const DefaultEpisodeDuration = 24

// Title regroupe les variantes de titre d'un média.
type Title struct {
	Romaji  string `json:"romaji"`
	English string `json:"english"`
	Native  string `json:"native"`
}

// Preferred applique l'ordre de préférence du bot : romaji, anglais, natif.
func (t Title) Preferred() string {
	switch {
	case t.Romaji != "":
		return t.Romaji
	case t.English != "":
		return t.English
	case t.Native != "":
		return t.Native
	default:
		return TitreInconnu
	}
}

// Media est l'anime concerné par une activité.
type Media struct {
	ID       int    `json:"id"`
	Title    Title  `json:"title"`
	Duration *int   `json:"duration"`
	Status   string `json:"status"`
	Episodes *int   `json:"episodes"`
}

// EpisodeDuration renvoie la durée brute d'un épisode en minutes.
func (m *Media) EpisodeDuration() int {
	if m == nil || m.Duration == nil || *m.Duration <= 0 {
		return DefaultEpisodeDuration
	}
	return *m.Duration
}

// EpisodeCount renvoie le nombre total d'épisodes connu, 0 si inconnu.
func (m *Media) EpisodeCount() int {
	if m == nil || m.Episodes == nil || *m.Episodes < 0 {
		return 0
	}
	return *m.Episodes
}

// PreferredTitle est une commodité sûre sur un média éventuellement nil.
func (m *Media) PreferredTitle() string {
	if m == nil {
		return TitreInconnu
	}
	return m.Title.Preferred()
}

// MediaID identifie le média, ou "noid" en son absence — même clé de repli que
// le bot JS, pour que les activités sans média soient regroupées ensemble.
func (m *Media) MediaID() string {
	if m == nil || m.ID == 0 {
		return "noid"
	}
	return strconv.Itoa(m.ID)
}

// Progress est le champ `progress` d'une activité AniList.
//
// L'API le renvoie sous forme de chaîne ("3", "1 - 3"), les jeux de test
// historiques sous forme de nombre, et il vaut null lors d'un simple changement
// de statut. On conserve la forme textuelle : les plages d'épisodes en dépendent.
type Progress struct {
	Valid bool
	Text  string
}

// NewProgress construit une valeur renseignée.
func NewProgress(s string) Progress { return Progress{Valid: true, Text: s} }

// UnmarshalJSON accepte une chaîne, un nombre ou null.
func (p *Progress) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*p = Progress{}
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*p = Progress{Valid: true, Text: s}
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*p = Progress{Valid: true, Text: n.String()}
	return nil
}

// MarshalJSON restitue une chaîne, ou null si la valeur est absente.
func (p Progress) MarshalJSON() ([]byte, error) {
	if !p.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(p.Text)
}

// Activity est une entrée de type ListActivity du flux d'un utilisateur.
type Activity struct {
	// ID est l'identifiant AniList de l'activité. Il vaut 0 pour les entrées
	// d'un autre type que ListActivity, que le fragment GraphQL renvoie vides.
	ID        int      `json:"id"`
	CreatedAt int64    `json:"createdAt"`
	Status    string   `json:"status"`
	Progress  Progress `json:"progress"`
	Media     *Media   `json:"media"`
}

// MediaListEntry est une entrée de MediaListCollection.
//
// Ce format n'est plus interrogé par le bot, mais le moteur de statistiques sait
// encore l'exploiter — c'est ce que couvrent les tests hérités.
type MediaListEntry struct {
	Media     *Media   `json:"media"`
	Progress  Progress `json:"progress"`
	UpdatedAt int64    `json:"updatedAt"`
}

// MediaList est une liste nommée (CURRENT, COMPLETED, …).
type MediaList struct {
	Entries []MediaListEntry `json:"entries"`
}
