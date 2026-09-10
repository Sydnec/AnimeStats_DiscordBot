package anilist

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// perPageDefault est la taille de page demandée à AniList.
//
// L'API plafonne perPage à 50 : demander davantage n'apportait rien et
// dépendait du bon vouloir du serveur, qui est libre de refuser la requête
// plutôt que de rogner la valeur en silence.
const perPageDefault = 50

// DefaultLookback est la marge d'historique récupérée au-delà du début de la
// période demandée. Le moteur de statistiques compte des deltas de progression :
// il lui faut la dernière progression connue *avant* la fenêtre, faute de quoi
// un anime déjà commencé serait recompté depuis le premier épisode.
//
// Six mois couvrent largement une saison de 12 épisodes étalée sur trois mois.
const DefaultLookback = 180 * 24 * time.Hour

const userIDQuery = `
query ($name: String) {
  User(name: $name) {
    id
  }
}`

// activitiesQuery reprend la requête du bot JS en y ajoutant deux choses :
// un tri explicite (le bot d'origine s'en remettait au défaut du serveur) et
// l'identifiant d'activité, qui sert de clé de déduplication entre les pages.
const activitiesQuery = `
query ($userId: Int, $page: Int, $perPage: Int) {
  Page(page: $page, perPage: $perPage) {
    pageInfo { hasNextPage currentPage lastPage }
    activities(userId: $userId, type: ANIME_LIST, sort: [ID_DESC]) {
      ... on ListActivity {
        id
        createdAt
        status
        progress
        media {
          id
          status
          episodes
          duration
          title { romaji english native }
        }
      }
    }
  }
}`

type userIDData struct {
	User *struct {
		ID int `json:"id"`
	} `json:"User"`
}

type activitiesData struct {
	Page struct {
		PageInfo struct {
			HasNextPage bool `json:"hasNextPage"`
			CurrentPage int  `json:"currentPage"`
			LastPage    int  `json:"lastPage"`
		} `json:"pageInfo"`
		Activities []Activity `json:"activities"`
	} `json:"Page"`
}

// UserID résout l'identifiant numérique d'un pseudo AniList.
func (c *Client) UserID(ctx context.Context, username string) (int, error) {
	var data userIDData
	if err := c.execute(ctx, userIDQuery, map[string]any{"name": username}, &data); err != nil {
		return 0, err
	}
	if data.User == nil || data.User.ID == 0 {
		return 0, ErrUserNotFound
	}
	return data.User.ID, nil
}

// UserExists indique si un pseudo AniList est valide.
//
// Le bot JS interrogeait ici MediaListCollection, c'est-à-dire la liste
// d'animes complète de l'utilisateur, uniquement pour tester son existence.
// Résoudre l'identifiant suffit et coûte infiniment moins cher.
func (c *Client) UserExists(ctx context.Context, username string) (bool, error) {
	if strings.TrimSpace(username) == "" {
		return false, nil
	}
	_, err := c.UserID(ctx, username)
	if errors.Is(err, ErrUserNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// FetchOptions pilote la récupération des activités.
type FetchOptions struct {
	// Since est le début de la période analysée. La pagination s'arrête dès
	// que l'historique remonte suffisamment loin avant cette date.
	Since time.Time
	// Lookback est la marge d'historique au-delà de Since. 0 prend DefaultLookback.
	Lookback time.Duration
}

// Activities récupère le flux d'activités d'un utilisateur.
//
// Toutes les activités récupérées sont renvoyées, y compris celles antérieures
// à Since : le moteur de statistiques s'en sert pour connaître la progression
// de départ, et filtre lui-même sur la période.
func (c *Client) Activities(ctx context.Context, username string, opts FetchOptions) ([]Activity, error) {
	lookback := opts.Lookback
	if lookback <= 0 {
		lookback = DefaultLookback
	}
	cutoff := opts.Since.Add(-lookback)

	if acts, ok := c.cached(username, cutoff); ok {
		c.log.Debug("activités AniList servies depuis le cache", "utilisateur", username, "nombre", len(acts))
		return acts, nil
	}

	userID, err := c.UserID(ctx, username)
	if err != nil {
		return nil, err
	}

	var (
		all       []Activity
		seen      = make(map[int]struct{})
		oldest    = int64(math.MaxInt64)
		prevPage  = int64(math.MaxInt64)
		monotonic = true
		exhausted bool
	)

	for page := 1; page <= c.maxPages; page++ {
		var data activitiesData
		vars := map[string]any{"userId": userID, "page": page, "perPage": perPageDefault}
		if err := c.execute(ctx, activitiesQuery, vars, &data); err != nil {
			return nil, fmt.Errorf("récupération des activités de %s (page %d) : %w", username, page, err)
		}

		pageOldest := int64(math.MaxInt64)
		kept := 0
		for _, a := range data.Page.Activities {
			// Le flux est une union : les activités d'un autre type que
			// ListActivity arrivent comme objets vides.
			if a.CreatedAt == 0 {
				continue
			}
			if a.ID != 0 {
				if _, dup := seen[a.ID]; dup {
					continue
				}
				seen[a.ID] = struct{}{}
			}
			all = append(all, a)
			kept++
			if a.CreatedAt < pageOldest {
				pageOldest = a.CreatedAt
			}
		}
		if pageOldest < oldest {
			oldest = pageOldest
		}

		if kept == 0 || !data.Page.PageInfo.HasNextPage {
			exhausted = true
			break
		}

		// L'arrêt anticipé suppose un flux trié du plus récent au plus ancien.
		// Si cette hypothèse se révèle fausse, on retombe sur l'ancien
		// comportement — parcourir jusqu'à MaxPages — plutôt que de tronquer
		// des données en silence.
		if pageOldest > prevPage {
			if monotonic {
				c.log.Warn("flux d'activités AniList non trié : arrêt anticipé désactivé", "utilisateur", username)
			}
			monotonic = false
		}
		prevPage = pageOldest

		if monotonic && pageOldest <= cutoff.Unix() {
			break
		}
	}

	c.log.Debug("activités AniList récupérées",
		"utilisateur", username, "nombre", len(all), "historique_complet", exhausted)

	c.store(username, all, oldest, exhausted)
	return all, nil
}

func (c *Client) cached(username string, cutoff time.Time) ([]Activity, bool) {
	if c.cacheTTL <= 0 {
		return nil, false
	}
	key := strings.ToLower(username)

	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.cache[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expiresAt) {
		delete(c.cache, key)
		return nil, false
	}
	// L'entrée n'est réutilisable que si elle remonte au moins aussi loin que
	// ce que réclame la période demandée.
	if !e.exhausted && e.since.After(cutoff) {
		return nil, false
	}
	out := make([]Activity, len(e.activities))
	copy(out, e.activities)
	return out, true
}

func (c *Client) store(username string, acts []Activity, oldest int64, exhausted bool) {
	if c.cacheTTL <= 0 {
		return
	}
	since := time.Unix(0, 0)
	if oldest != math.MaxInt64 {
		since = time.Unix(oldest, 0)
	}
	stored := make([]Activity, len(acts))
	copy(stored, acts)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[strings.ToLower(username)] = cacheEntry{
		expiresAt:  time.Now().Add(c.cacheTTL),
		since:      since,
		exhausted:  exhausted,
		activities: stored,
	}
}
