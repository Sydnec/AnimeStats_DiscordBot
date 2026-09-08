package anilist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Endpoint est l'API GraphQL publique d'AniList (aucune authentification).
const Endpoint = "https://graphql.anilist.co"

// ErrUserNotFound signale un pseudo AniList inexistant.
var ErrUserNotFound = errors.New("utilisateur AniList introuvable")

const (
	// AniList annonce 90 requêtes/minute mais fonctionne depuis longtemps en
	// mode dégradé à 30/minute. On se cale sur la valeur basse.
	defaultRatePerMinute = 30
	defaultBurst         = 5
	defaultAttempts      = 4
	defaultTimeout       = 30 * time.Second
	perPage              = 100
	// Pages supplémentaires récupérées au-delà de la date de début : elles
	// fournissent la progression antérieure nécessaire au calcul des deltas.
	lookbehindPages = 1
)

// DefaultUserAgent identifie le bot auprès d'AniList.
const DefaultUserAgent = "animestats-bot (+https://github.com/Sydnec/AnimeStats_DiscordBot)"

// Options configure un Client. Tous les champs sont facultatifs.
type Options struct {
	Endpoint   string
	UserAgent  string
	HTTPClient *http.Client
	Logger     *slog.Logger
	MaxPages   int
	CacheTTL   time.Duration
	// RatePerMinute plafonne le débit sortant. 0 conserve la valeur par défaut,
	// une valeur négative désactive la limitation (utile en test).
	RatePerMinute int
	Attempts      int
}

// Client interroge AniList en respectant un débit maximal, avec réessais.
type Client struct {
	endpoint  string
	userAgent string
	http      *http.Client
	log       *slog.Logger
	limiter   *rate.Limiter
	maxPages  int
	attempts  int

	cacheTTL time.Duration
	mu       sync.Mutex
	cache    map[string]cacheEntry
}

type cacheEntry struct {
	expiresAt time.Time
	// since est l'horodatage de l'activité la plus ancienne mémorisée.
	since time.Time
	// exhausted indique que tout l'historique disponible a été récupéré.
	exhausted  bool
	activities []Activity
}

// New construit un client prêt à l'emploi.
func New(opts Options) *Client {
	c := &Client{
		endpoint:  opts.Endpoint,
		userAgent: opts.UserAgent,
		http:      opts.HTTPClient,
		log:       opts.Logger,
		maxPages:  opts.MaxPages,
		attempts:  opts.Attempts,
		cacheTTL:  opts.CacheTTL,
		cache:     make(map[string]cacheEntry),
	}
	if c.endpoint == "" {
		c.endpoint = Endpoint
	}
	if c.userAgent == "" {
		c.userAgent = DefaultUserAgent
	}
	if c.http == nil {
		c.http = &http.Client{Timeout: defaultTimeout}
	}
	if c.log == nil {
		c.log = slog.New(slog.DiscardHandler)
	}
	if c.maxPages <= 0 {
		c.maxPages = 20
	}
	if c.attempts <= 0 {
		c.attempts = defaultAttempts
	}
	switch {
	case opts.RatePerMinute < 0:
		c.limiter = rate.NewLimiter(rate.Inf, 0)
	case opts.RatePerMinute == 0:
		c.limiter = rate.NewLimiter(rate.Limit(defaultRatePerMinute)/60, defaultBurst)
	default:
		c.limiter = rate.NewLimiter(rate.Limit(opts.RatePerMinute)/60, defaultBurst)
	}
	return c
}

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type graphQLError struct {
	Message string `json:"message"`
	Status  int    `json:"status"`
}

type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphQLError  `json:"errors"`
}

// retryableError marque une erreur pour laquelle un nouvel essai a du sens.
type retryableError struct {
	err   error
	after time.Duration
}

func (e *retryableError) Error() string { return e.err.Error() }
func (e *retryableError) Unwrap() error { return e.err }

// execute envoie une requête GraphQL et désérialise `data` dans out.
func (c *Client) execute(ctx context.Context, query string, variables map[string]any, out any) error {
	body, err := json.Marshal(graphQLRequest{Query: query, Variables: variables})
	if err != nil {
		return fmt.Errorf("sérialisation de la requête : %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= c.attempts; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return err
		}

		err := c.attempt(ctx, body, out)
		if err == nil {
			return nil
		}
		lastErr = err

		var retry *retryableError
		if !errors.As(err, &retry) || attempt == c.attempts {
			return err
		}

		delay := retry.after
		if delay <= 0 {
			// Repli exponentiel : 1s, 2s, 4s…
			delay = time.Duration(1<<(attempt-1)) * time.Second
		}
		c.log.Warn("requête AniList à réessayer",
			"tentative", attempt, "sur", c.attempts, "attente", delay, "erreur", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}

func (c *Client) attempt(ctx context.Context, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("construction de la requête : %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	res, err := c.http.Do(req)
	if err != nil {
		// Panne réseau ou délai dépassé : cela vaut la peine de réessayer.
		return &retryableError{err: fmt.Errorf("appel AniList : %w", err)}
	}
	defer res.Body.Close()

	// 1 Mio suffit très largement pour une page de 100 activités et borne la
	// mémoire en cas de réponse anormale.
	payload, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return &retryableError{err: fmt.Errorf("lecture de la réponse AniList : %w", err)}
	}

	if res.StatusCode == http.StatusTooManyRequests {
		return &retryableError{
			err:   fmt.Errorf("AniList a renvoyé 429 (quota dépassé)"),
			after: retryAfter(res.Header),
		}
	}
	if res.StatusCode >= 500 {
		return &retryableError{err: fmt.Errorf("AniList a renvoyé %s", res.Status)}
	}

	var parsed graphQLResponse
	if err := json.Unmarshal(payload, &parsed); err != nil {
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("AniList a renvoyé %s : %s", res.Status, truncate(string(payload), 200))
		}
		return fmt.Errorf("réponse AniList illisible : %w", err)
	}

	if len(parsed.Errors) > 0 {
		msgs := make([]string, 0, len(parsed.Errors))
		notFound := false
		for _, e := range parsed.Errors {
			msgs = append(msgs, e.Message)
			if e.Status == http.StatusNotFound || strings.EqualFold(e.Message, "Not Found.") ||
				strings.EqualFold(e.Message, "Not Found") {
				notFound = true
			}
		}
		if notFound {
			return ErrUserNotFound
		}
		return fmt.Errorf("erreur GraphQL AniList : %s", strings.Join(msgs, " ; "))
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("AniList a renvoyé %s", res.Status)
	}
	if out == nil || len(parsed.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(parsed.Data, out); err != nil {
		return fmt.Errorf("réponse AniList inattendue : %w", err)
	}
	return nil
}

// retryAfter interprète l'en-tête Retry-After (secondes ou date HTTP).
func retryAfter(h http.Header) time.Duration {
	raw := strings.TrimSpace(h.Get("Retry-After"))
	if raw == "" {
		return 0
	}
	if secs, err := strconv.Atoi(raw); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(raw); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
