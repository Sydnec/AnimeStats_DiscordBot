// Package bot expose le bot Discord : session, commandes et envoi de MP.
package bot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/Sydnec/AnimeStats_DiscordBot/internal/anilist"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/config"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/report"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/store"
)

// commandsHashKey mémorise l'empreinte des commandes déjà publiées.
const commandsHashKey = "commands_hash"

// Bot relie la session Discord aux autres composants.
type Bot struct {
	cfg     config.Config
	log     *slog.Logger
	session *discordgo.Session
	store   *store.Store
	anilist *anilist.Client
	reports *report.Service

	// ctx borne la durée de vie du bot : il est annulé à l'arrêt du service,
	// ce qui interrompt les commandes encore en cours.
	ctx context.Context
}

// New construit le bot. La session n'est pas encore ouverte.
func New(cfg config.Config, log *slog.Logger, st *store.Store, cl *anilist.Client) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("création de la session Discord : %w", err)
	}
	// Aucun intent n'est nécessaire : les interactions ne sont pas filtrées par
	// les intents, et le bot ne lit aucun message. Désactiver le cache d'état
	// évite d'entretenir en mémoire des serveurs dont on n'a que faire.
	session.Identify.Intents = discordgo.IntentsNone
	session.StateEnabled = false

	b := &Bot{cfg: cfg, log: log, session: session, store: st, anilist: cl}
	b.reports = &report.Service{
		AniList: cl,
		Store:   st,
		DM:      b,
		Log:     log,
		Stats:   cfg.StatsOptions(),
	}

	session.AddHandler(b.onReady)
	session.AddHandler(b.onInteraction)
	return b, nil
}

// Reports donne accès au service de récapitulatif, utilisé par le planificateur.
func (b *Bot) Reports() *report.Service { return b.reports }

// baseContext renvoie le contexte de vie du bot.
func (b *Bot) baseContext() context.Context {
	if b.ctx == nil {
		return context.Background()
	}
	return b.ctx
}

// Start ouvre la session et publie les commandes si nécessaire.
func (b *Bot) Start(ctx context.Context) error {
	b.ctx = ctx
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("connexion à Discord : %w", err)
	}
	if err := b.syncCommands(ctx); err != nil {
		// Une publication ratée n'empêche pas le bot de répondre aux commandes
		// déjà enregistrées : on journalise sans interrompre le démarrage.
		b.log.Error("publication des commandes impossible", "erreur", err)
	}
	return nil
}

// Close ferme proprement la session.
func (b *Bot) Close() error { return b.session.Close() }

func (b *Bot) onReady(s *discordgo.Session, r *discordgo.Ready) {
	b.log.Info("connecté à Discord",
		"utilisateur", r.User.Username, "identifiant", r.User.ID)
}

// syncCommands publie les commandes uniquement si leur définition a changé.
//
// Republier à chaque démarrage consomme le quota de création et impose un
// délai de propagation ; l'empreinte évite les appels inutiles, notamment
// quand systemd redémarre le service en boucle.
func (b *Bot) syncCommands(ctx context.Context) error {
	defs := commandDefinitions()

	payload, err := json.Marshal(struct {
		Guild    string                          `json:"guild"`
		Commands []*discordgo.ApplicationCommand `json:"commands"`
	}{Guild: b.cfg.DevGuildID, Commands: defs})
	if err != nil {
		return fmt.Errorf("sérialisation des commandes : %w", err)
	}
	sum := sha256.Sum256(payload)
	hash := hex.EncodeToString(sum[:])

	previous, found, err := b.store.GetMeta(ctx, commandsHashKey)
	if err != nil {
		b.log.Warn("empreinte des commandes illisible, publication forcée", "erreur", err)
	} else if found && previous == hash {
		b.log.Info("commandes déjà à jour", "nombre", len(defs))
		return nil
	}

	// Une guilde de développement rend la publication immédiate, alors que le
	// périmètre global peut mettre jusqu'à une heure à se propager.
	scope := b.cfg.DevGuildID
	if _, err := b.session.ApplicationCommandBulkOverwrite(
		b.session.State.User.ID, scope, defs, discordgo.WithContext(ctx)); err != nil {
		return fmt.Errorf("publication des commandes : %w", err)
	}

	if scope == "" {
		b.log.Info("commandes publiées globalement", "nombre", len(defs))
	} else {
		b.log.Info("commandes publiées sur une guilde", "guilde", scope, "nombre", len(defs))
	}

	if err := b.store.SetMeta(ctx, commandsHashKey, hash); err != nil {
		b.log.Warn("empreinte des commandes non mémorisée", "erreur", err)
	}
	return nil
}

// interactionTimeout borne le traitement d'une commande. Discord accepte un
// message de suivi pendant quinze minutes ; rester bien en deçà évite qu'une
// commande bloquée n'immobilise une goroutine indéfiniment.
const interactionTimeout = 2 * time.Minute
