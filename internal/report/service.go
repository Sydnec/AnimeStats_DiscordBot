// Package report orchestre la production d'un récapitulatif : récupération des
// activités AniList, calcul, mise en forme et envoi en message privé.
package report

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Sydnec/AnimeStats_DiscordBot/internal/anilist"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/period"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/render"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/stats"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/store"
)

// DMSender envoie un message privé à un utilisateur Discord.
type DMSender interface {
	SendDM(ctx context.Context, userID, content string) error
}

// Service assemble les dépendances nécessaires à un récapitulatif.
type Service struct {
	AniList *anilist.Client
	Store   *store.Store
	DM      DMSender
	Log     *slog.Logger
	Stats   stats.Options
}

// Compose produit le message d'un récapitulatif sans l'envoyer.
func (s *Service) Compose(ctx context.Context, anilistUsername string, p period.Period) (string, error) {
	activities, err := s.AniList.Activities(ctx, anilistUsername, anilist.FetchOptions{Since: p.Start})
	if err != nil {
		return "", err
	}

	res := stats.Compute(stats.Input{Activities: activities}, p.Start, p.End, s.Stats)

	days := p.Days
	if days < 1 {
		days = 1
	}

	return render.Report{
		Header:           p.Header,
		TotalMinutes:     res.TotalMinutes,
		AvgMinutesPerDay: res.TotalMinutes / float64(days),
		Episodes:         res.TotalEpisodes,
		TopDay:           res.TopDay,
		TopMinutes:       res.TopMinutes,
		ShowTopDay:       p.ShowTopDay,
		Titles:           res.Titles,
	}.Message(), nil
}

// Send compose puis envoie le récapitulatif en message privé.
func (s *Service) Send(ctx context.Context, discordUserID, anilistUsername string, p period.Period) error {
	if anilistUsername == "" {
		return errors.New("aucun pseudo AniList associé")
	}

	content, err := s.Compose(ctx, anilistUsername, p)
	if err != nil {
		return fmt.Errorf("calcul du récapitulatif de %s : %w", anilistUsername, err)
	}
	if err := s.DM.SendDM(ctx, discordUserID, content); err != nil {
		return fmt.Errorf("envoi du récapitulatif à %s : %w", discordUserID, err)
	}
	return nil
}

// Broadcast envoie le récapitulatif à tous les abonnés d'une fréquence.
//
// Les envois sont séquentiels — c'était déjà le cas du bot JS, et c'est de
// toute façon la bonne approche face au quota d'AniList. Une erreur sur un
// abonné est journalisée sans interrompre les suivants.
func (s *Service) Broadcast(ctx context.Context, freq store.Frequency, p period.Period) (sent, failed int) {
	log := s.Log.With("tâche", string(freq), "période", p.Key)

	followers, err := s.Store.ListByFrequency(ctx, freq)
	if err != nil {
		log.Error("lecture des abonnés impossible", "erreur", err)
		return 0, 0
	}
	if len(followers) == 0 {
		log.Info("aucun abonné")
		return 0, 0
	}

	log.Info("envoi des récapitulatifs", "abonnés", len(followers))
	for _, f := range followers {
		if ctx.Err() != nil {
			log.Warn("envoi interrompu", "envoyés", sent, "restants", len(followers)-sent-failed)
			return sent, failed
		}
		if err := s.Send(ctx, f.UserID, f.AniListUsername, p); err != nil {
			failed++
			log.Error("échec d'envoi", "utilisateur", f.UserID, "pseudo", f.AniListUsername, "erreur", err)
			continue
		}
		sent++
	}
	log.Info("récapitulatifs envoyés", "succès", sent, "échecs", failed)
	return sent, failed
}

// SendNow envoie un récapitulatif pour les N derniers jours.
func (s *Service) SendNow(ctx context.Context, discordUserID, anilistUsername string, days int, now time.Time, loc *time.Location) error {
	return s.Send(ctx, discordUserID, anilistUsername, period.LastDays(now, loc, days))
}
