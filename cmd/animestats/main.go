// Commande animestats : bot Discord qui envoie en message privé des
// récapitulatifs de statistiques de visionnage AniList.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	// Embarque la base des fuseaux horaires dans le binaire : un conteneur LXC
	// minimal n'a pas forcément /usr/share/zoneinfo, et le bot y perdrait ses
	// bornes de période.
	_ "time/tzdata"

	"github.com/Sydnec/AnimeStats_DiscordBot/internal/anilist"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/bot"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/config"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/logging"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/period"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/scheduler"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/store"
)

// version est renseignée à la compilation via -ldflags.
var version = "dev"

// shutdownTimeout borne l'attente des tâches en cours à l'arrêt.
const shutdownTimeout = 20 * time.Second

func main() {
	var (
		showVersion = flag.Bool("version", false, "affiche la version puis quitte")
		check       = flag.Bool("check", false, "valide la configuration et la base, puis quitte sans se connecter à Discord")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("animestats", version)
		return
	}

	if err := run(*check); err != nil {
		fmt.Fprintln(os.Stderr, "animestats:", err)
		os.Exit(1)
	}
}

func run(checkOnly bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := logging.New(os.Stderr, cfg.LogLevel)
	log.Info("démarrage", "version", version, "configuration", cfg)

	// Le contexte est annulé sur SIGINT ou SIGTERM, ce que systemd envoie à
	// l'arrêt comme au redémarrage du service.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := st.Close(); err != nil {
			log.Error("fermeture de la base", "erreur", err)
		}
	}()

	if checkOnly {
		followers, err := st.ListAll(ctx)
		if err != nil {
			return err
		}
		log.Info("vérification réussie", "base", cfg.DBPath, "abonnés", len(followers))
		return nil
	}

	client := anilist.New(anilist.Options{
		Logger:    log.With("composant", "anilist"),
		MaxPages:  cfg.AniListMaxPages,
		CacheTTL:  cfg.AniListCacheTTL,
		UserAgent: fmt.Sprintf("animestats-bot/%s (+https://github.com/Sydnec/AnimeStats_DiscordBot)", version),
	})

	b, err := bot.New(cfg, log.With("composant", "discord"), st, client)
	if err != nil {
		return err
	}
	if err := b.Start(ctx); err != nil {
		return err
	}
	defer func() {
		if err := b.Close(); err != nil {
			log.Error("fermeture de la session Discord", "erreur", err)
		}
	}()

	sched := scheduler.New(scheduler.Options{
		Store:    st,
		Reports:  b.Reports(),
		Log:      log.With("composant", "planificateur"),
		Location: cfg.Location,
		CatchUp:  cfg.CatchUpMissedRuns,
	}, []scheduler.Job{
		{
			Key:   "monthly",
			Spec:  cfg.MonthlyCron,
			Freq:  store.Monthly,
			Build: period.PreviousMonth,
		},
		{
			Key:   "yearly",
			Spec:  cfg.YearlyCron,
			Freq:  store.Yearly,
			Build: period.PreviousYear,
		},
	})
	if err := sched.Start(ctx); err != nil {
		return err
	}

	log.Info("bot en fonctionnement")
	<-ctx.Done()

	// Le contexte est déjà annulé : on cesse d'écouter les signaux pour qu'un
	// second Ctrl-C interrompe brutalement si l'arrêt propre traîne.
	stop()
	log.Info("arrêt en cours")
	sched.Stop(shutdownTimeout)

	if err := ctx.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
