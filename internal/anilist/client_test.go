package anilist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient construit un client sans limitation de débit ni attente.
func newTestClient(t *testing.T, h http.HandlerFunc, opts Options) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	opts.Endpoint = srv.URL
	opts.RatePerMinute = -1 // désactive le limiteur
	return New(opts)
}

// readQuery renvoie le corps GraphQL décodé de la requête entrante.
func readQuery(t *testing.T, r *http.Request) (string, map[string]any) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("lecture du corps : %v", err)
	}
	var req struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("corps illisible : %v", err)
	}
	return req.Query, req.Variables
}

func TestUserID(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); !strings.Contains(got, "animestats-bot") {
			t.Errorf("User-Agent = %q", got)
		}
		fmt.Fprint(w, `{"data":{"User":{"id":4242}}}`)
	}, Options{})

	id, err := c.UserID(context.Background(), "Sydnec")
	if err != nil {
		t.Fatalf("UserID : %v", err)
	}
	if id != 4242 {
		t.Errorf("UserID = %d, attendu 4242", id)
	}
}

func TestUserNotFound(t *testing.T) {
	// AniList répond 404 avec un tableau errors quand le pseudo n'existe pas.
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"data":null,"errors":[{"message":"Not Found.","status":404}]}`)
	}, Options{})

	if _, err := c.UserID(context.Background(), "personne"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("UserID = %v, attendu ErrUserNotFound", err)
	}

	exists, err := c.UserExists(context.Background(), "personne")
	if err != nil {
		t.Fatalf("UserExists : %v", err)
	}
	if exists {
		t.Error("UserExists aurait dû renvoyer false")
	}
}

func TestUserExistsRejectsEmpty(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("aucune requête ne devrait partir pour un pseudo vide")
	}, Options{})

	exists, err := c.UserExists(context.Background(), "   ")
	if err != nil || exists {
		t.Errorf("UserExists(\"   \") = %v, %v", exists, err)
	}
}

// activityJSON produit une activité de test.
func activityJSON(id int, createdAt int64, progress string) string {
	return fmt.Sprintf(
		`{"id":%d,"createdAt":%d,"status":"watched episode","progress":%q,`+
			`"media":{"id":1,"status":"FINISHED","episodes":12,"duration":24,`+
			`"title":{"romaji":"Test","english":null,"native":null}}}`,
		id, createdAt, progress)
}

func TestActivitiesPagination(t *testing.T) {
	now := time.Now()
	// Trois pages d'une activité chacune, du plus récent au plus ancien.
	ages := []time.Duration{1 * time.Hour, 12 * time.Hour, 60 * time.Hour}

	var pages int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query, vars := readQuery(t, r)
		if strings.Contains(query, "User(name") {
			fmt.Fprint(w, `{"data":{"User":{"id":1}}}`)
			return
		}
		if !strings.Contains(query, "sort: [ID_DESC]") {
			t.Error("la requête devrait trier explicitement les activités")
		}
		atomic.AddInt32(&pages, 1)
		page := int(vars["page"].(float64))
		created := now.Add(-ages[page-1]).Unix()
		hasNext := page < len(ages)
		fmt.Fprintf(w, `{"data":{"Page":{"pageInfo":{"hasNextPage":%t,"currentPage":%d,"lastPage":%d},"activities":[%s]}}}`,
			hasNext, page, len(ages), activityJSON(page, created, "3"))
	}, Options{MaxPages: 10})

	// La fenêtre demandée démarre il y a 24 h, avec 24 h de marge d'historique :
	// la troisième page (il y a 96 h) doit déclencher l'arrêt.
	acts, err := c.Activities(context.Background(), "Sydnec", FetchOptions{
		Since:    now.Add(-24 * time.Hour),
		Lookback: 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("Activities : %v", err)
	}
	if got := atomic.LoadInt32(&pages); got != 3 {
		t.Errorf("%d pages récupérées, attendu 3", got)
	}
	if len(acts) != 3 {
		t.Fatalf("%d activités, attendu 3", len(acts))
	}
	// Les activités antérieures à la fenêtre sont conservées : le moteur en a
	// besoin pour connaître la progression de départ.
	if acts[2].CreatedAt >= now.Add(-24*time.Hour).Unix() {
		t.Error("l'activité la plus ancienne aurait dû être conservée")
	}
}

func TestActivitiesStopsAtMaxPages(t *testing.T) {
	now := time.Now()
	var pages int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query, vars := readQuery(t, r)
		if strings.Contains(query, "User(name") {
			fmt.Fprint(w, `{"data":{"User":{"id":1}}}`)
			return
		}
		atomic.AddInt32(&pages, 1)
		page := int(vars["page"].(float64))
		// Toujours une page suivante, toujours des activités récentes.
		fmt.Fprintf(w, `{"data":{"Page":{"pageInfo":{"hasNextPage":true,"currentPage":%d,"lastPage":999},"activities":[%s]}}}`,
			page, activityJSON(page, now.Add(-time.Duration(page)*time.Minute).Unix(), "1"))
	}, Options{MaxPages: 4})

	if _, err := c.Activities(context.Background(), "Sydnec", FetchOptions{Since: now.Add(-365 * 24 * time.Hour)}); err != nil {
		t.Fatalf("Activities : %v", err)
	}
	if got := atomic.LoadInt32(&pages); got != 4 {
		t.Errorf("%d pages récupérées, attendu 4 (plafond MaxPages)", got)
	}
}

func TestActivitiesDeduplicatesAcrossPages(t *testing.T) {
	now := time.Now()
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query, vars := readQuery(t, r)
		if strings.Contains(query, "User(name") {
			fmt.Fprint(w, `{"data":{"User":{"id":1}}}`)
			return
		}
		page := int(vars["page"].(float64))
		// Les deux pages renvoient la même activité (id 7).
		fmt.Fprintf(w, `{"data":{"Page":{"pageInfo":{"hasNextPage":%t,"currentPage":%d,"lastPage":2},"activities":[%s]}}}`,
			page == 1, page, activityJSON(7, now.Add(-time.Hour).Unix(), "3"))
	}, Options{MaxPages: 5})

	acts, err := c.Activities(context.Background(), "Sydnec", FetchOptions{Since: now.Add(-2 * time.Hour)})
	if err != nil {
		t.Fatalf("Activities : %v", err)
	}
	if len(acts) != 1 {
		t.Errorf("%d activités, attendu 1 après déduplication", len(acts))
	}
}

func TestActivitiesIgnoresNonListActivities(t *testing.T) {
	now := time.Now()
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query, _ := readQuery(t, r)
		if strings.Contains(query, "User(name") {
			fmt.Fprint(w, `{"data":{"User":{"id":1}}}`)
			return
		}
		// Le flux est une union : les autres types d'activités arrivent vides.
		fmt.Fprintf(w, `{"data":{"Page":{"pageInfo":{"hasNextPage":false},"activities":[{},%s,{}]}}}`,
			activityJSON(1, now.Add(-time.Hour).Unix(), "3"))
	}, Options{})

	acts, err := c.Activities(context.Background(), "Sydnec", FetchOptions{Since: now.Add(-2 * time.Hour)})
	if err != nil {
		t.Fatalf("Activities : %v", err)
	}
	if len(acts) != 1 {
		t.Fatalf("%d activités, attendu 1", len(acts))
	}
	if acts[0].Media == nil {
		t.Error("le média de l'activité valide aurait dû être renseigné")
	}
}

func TestRetryOn429(t *testing.T) {
	var calls int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, `{"data":{"User":{"id":7}}}`)
	}, Options{})

	id, err := c.UserID(context.Background(), "Sydnec")
	if err != nil {
		t.Fatalf("UserID : %v", err)
	}
	if id != 7 {
		t.Errorf("UserID = %d", id)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("%d appels, attendu 2 (un 429 puis un succès)", got)
	}
}

func TestRetryOn5xxThenGivesUp(t *testing.T) {
	var calls int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadGateway)
	}, Options{Attempts: 2})

	if _, err := c.UserID(context.Background(), "Sydnec"); err == nil {
		t.Fatal("une erreur était attendue après épuisement des tentatives")
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("%d appels, attendu 2", got)
	}
}

func TestGraphQLErrorIsReported(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":null,"errors":[{"message":"Champ invalide","status":400}]}`)
	}, Options{})

	_, err := c.UserID(context.Background(), "Sydnec")
	if err == nil || !strings.Contains(err.Error(), "Champ invalide") {
		t.Fatalf("erreur = %v, le message GraphQL était attendu", err)
	}
}

func TestCacheReuseAndScope(t *testing.T) {
	now := time.Now()
	var pageCalls int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query, _ := readQuery(t, r)
		if strings.Contains(query, "User(name") {
			fmt.Fprint(w, `{"data":{"User":{"id":1}}}`)
			return
		}
		atomic.AddInt32(&pageCalls, 1)
		fmt.Fprintf(w, `{"data":{"Page":{"pageInfo":{"hasNextPage":false},"activities":[%s]}}}`,
			activityJSON(1, now.Add(-10*24*time.Hour).Unix(), "3"))
	}, Options{CacheTTL: time.Minute})

	opts := FetchOptions{Since: now.Add(-24 * time.Hour), Lookback: time.Hour}
	if _, err := c.Activities(context.Background(), "Sydnec", opts); err != nil {
		t.Fatalf("premier appel : %v", err)
	}
	if _, err := c.Activities(context.Background(), "SYDNEC", opts); err != nil {
		t.Fatalf("second appel : %v", err)
	}
	// L'historique complet a été récupéré : la seconde demande est servie
	// depuis le cache, insensible à la casse du pseudo.
	if got := atomic.LoadInt32(&pageCalls); got != 1 {
		t.Errorf("%d requêtes de pagination, attendu 1 grâce au cache", got)
	}
}

func TestContextCancellation(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Lire le corps permet au serveur de détecter la déconnexion du client
		// et donc d'annuler r.Context() ; la borne de temps garantit que
		// srv.Close() ne reste pas bloqué si ce n'était pas le cas.
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}, Options{})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := c.UserID(ctx, "Sydnec"); err == nil {
		t.Fatal("une erreur était attendue à l'expiration du contexte")
	}
}
