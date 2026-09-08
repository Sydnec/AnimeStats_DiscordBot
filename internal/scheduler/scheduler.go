// Package scheduler déclenche les récapitulatifs périodiques.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/Sydnec/AnimeStats_DiscordBot/internal/period"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/report"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/store"
)

// Job décrit une tâche planifiée.
type Job struct {
	// Key identifie la tâche dans la table job_runs.
	Key string
	// Spec est l'expression cron à cinq champs, évaluée dans le fuseau configuré.
	Spec string
	// Freq sélectionne les abonnés concernés.
	Freq store.Frequency
	// Build construit la période à couvrir à un instant donné.
	Build func(now time.Time, loc *time.Location) period.Period
}

// Scheduler exécute les tâches planifiées.
type Scheduler struct {
	cron    *cron.Cron
	store   *store.Store
	reports *report.Service
	log     *slog.Logger
	loc     *time.Location
	jobs    []Job
	catchUp bool
}

// Options paramètre le planificateur.
type Options struct {
	Store    *store.Store
	Reports  *report.Service
	Log      *slog.Logger
	Location *time.Location
	// CatchUp rattrape au démarrage un envoi manqué, par exemple si la machine
	// était éteinte à l'heure prévue.
	CatchUp bool
}

// New construit un planificateur sans le démarrer.
func New(opts Options, jobs []Job) *Scheduler {
	log := opts.Log
	return &Scheduler{
		cron: cron.New(
			cron.WithLocation(opts.Location),
			cron.WithChain(
				// Une panique dans une tâche ne doit pas emporter le service,
				// et deux exécutions ne doivent jamais se chevaucher.
				cron.Recover(cronLogger{log}),
				cron.SkipIfStillRunning(cronLogger{log}),
			),
		),
		store:   opts.Store,
		reports: opts.Reports,
		log:     log,
		loc:     opts.Location,
		jobs:    jobs,
		catchUp: opts.CatchUp,
	}
}

// Start enregistre les tâches, rattrape un éventuel envoi manqué, puis lance
// le planificateur.
func (s *Scheduler) Start(ctx context.Context) error {
	for _, job := range s.jobs {
		schedule, err := cron.ParseStandard(job.Spec)
		if err != nil {
			return fmt.Errorf("expression cron %q de la tâche %s invalide : %w", job.Spec, job.Key, err)
		}

		s.cron.Schedule(schedule, cron.FuncJob(func() { s.run(ctx, job) }))
		s.log.Info("tâche planifiée", "tâche", job.Key, "expression", job.Spec,
			"prochaine", schedule.Next(time.Now().In(s.loc)).Format(time.RFC3339))

		if s.catchUp {
			s.catchUpJob(ctx, job, schedule)
		}
	}

	s.cron.Start()
	return nil
}

// Stop arrête le planificateur et attend la fin des tâches en cours.
func (s *Scheduler) Stop(timeout time.Duration) {
	stopped := s.cron.Stop()
	select {
	case <-stopped.Done():
	case <-time.After(timeout):
		s.log.Warn("tâches encore en cours à l'arrêt du planificateur")
	}
}

// run exécute une tâche et mémorise la période traitée.
func (s *Scheduler) run(ctx context.Context, job Job) {
	if ctx.Err() != nil {
		return
	}
	p := job.Build(time.Now(), s.loc)
	sent, failed := s.reports.Broadcast(ctx, job.Freq, p)

	// Une diffusion interrompue par l'arrêt du service n'est pas une diffusion
	// faite : ne pas marquer la période, pour que le rattrapage la reprenne au
	// prochain démarrage. Quelques abonnés déjà servis recevront un doublon,
	// ce qui vaut mieux que de les priver tous du récapitulatif.
	if ctx.Err() != nil {
		s.log.Warn("diffusion interrompue, la période sera reprise au prochain démarrage",
			"tâche", job.Key, "période", p.Key, "envoyés", sent, "échecs", failed)
		return
	}

	if err := s.store.MarkJobRun(ctx, job.Key, p.Key); err != nil {
		s.log.Error("suivi de la tâche non enregistré", "tâche", job.Key, "erreur", err)
	}
}

// catchUpJob rattrape la période courante si son heure de déclenchement est
// passée sans que la tâche ait tourné.
//
// Au tout premier démarrage, aucun envoi n'est rattrapé : la période en cours
// est simplement notée comme traitée. Sans cette précaution, une migration
// depuis l'ancien bot arroserait tous les abonnés dès le lancement.
func (s *Scheduler) catchUpJob(ctx context.Context, job Job, schedule cron.Schedule) {
	now := time.Now().In(s.loc)
	p := job.Build(now, s.loc)

	last, found, err := s.store.LastJobPeriod(ctx, job.Key)
	if err != nil {
		s.log.Error("suivi de la tâche illisible", "tâche", job.Key, "erreur", err)
		return
	}
	if found && last == p.Key {
		return
	}

	if !found {
		s.log.Info("premier démarrage : aucun rattrapage, la période courante est notée comme traitée",
			"tâche", job.Key, "période", p.Key)
		if err := s.store.MarkJobRun(ctx, job.Key, p.Key); err != nil {
			s.log.Error("suivi de la tâche non enregistré", "tâche", job.Key, "erreur", err)
		}
		return
	}

	// Le déclenchement qui produit la période courante a-t-il déjà eu lieu ?
	// On le sait en comparant la période qu'aurait produite le dernier
	// déclenchement à celle attendue maintenant.
	firedAt, ok := lastActivation(schedule, now)
	if !ok || job.Build(firedAt, s.loc).Key != p.Key {
		// L'heure d'envoi de la période courante n'est pas encore passée :
		// le planificateur s'en chargera le moment venu.
		return
	}

	s.log.Warn("envoi manqué, rattrapage en cours",
		"tâche", job.Key, "période", p.Key, "dernière_période", last,
		"déclenchement_prévu", firedAt.Format(time.RFC3339))
	s.run(ctx, job)
}

// lastActivation renvoie le dernier déclenchement du planning antérieur ou égal
// à now.
//
// robfig/cron ne sait avancer que vers le futur : on part donc d'un point du
// passé et on avance. La fenêtre de recherche s'élargit progressivement, ce qui
// garde le nombre d'itérations bas quelle que soit la fréquence — d'une
// expression à la minute jusqu'à une expression annuelle.
func lastActivation(schedule cron.Schedule, now time.Time) (time.Time, bool) {
	lookbacks := []time.Duration{
		2 * time.Hour,
		48 * time.Hour,
		40 * 24 * time.Hour,
		400 * 24 * time.Hour,
	}

	for _, back := range lookbacks {
		t := now.Add(-back)
		var last time.Time
		for i := 0; i < 2000; i++ {
			next := schedule.Next(t)
			if next.IsZero() || next.After(now) {
				break
			}
			last = next
			t = next
		}
		if !last.IsZero() {
			return last, true
		}
	}
	return time.Time{}, false
}

// cronLogger adapte slog à l'interface attendue par robfig/cron.
type cronLogger struct{ log *slog.Logger }

func (c cronLogger) Info(msg string, keysAndValues ...any) {
	c.log.Debug(msg, keysAndValues...)
}

func (c cronLogger) Error(err error, msg string, keysAndValues ...any) {
	c.log.Error(msg, append(keysAndValues, "erreur", err)...)
}
