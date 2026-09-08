package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/Sydnec/AnimeStats_DiscordBot/internal/mask"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/period"
	"github.com/Sydnec/AnimeStats_DiscordBot/internal/store"
)

// Messages adressés à l'utilisateur.
const (
	msgMaskInvalid   = "Masque invalide. Utilisez 2 caractères parmi 0, 1 et * (ex: 1*). Format : monthly, yearly."
	msgUserUnknown   = "Pseudo AniList invalide ou introuvable."
	msgUserCheckFail = "Impossible de vérifier ce pseudo AniList pour le moment. Réessayez dans quelques minutes."
	msgNoAccount     = "Aucun compte AniList associé au vôtre. Utilisez `/follow` pour vous abonner."
	msgUnfollowed    = "Vous ne recevez plus les récaps."
	msgNotFollowed   = "Vous n'étiez pas abonné."
	msgInternal      = "Erreur interne. Réessayez plus tard."
	msgDMRefused     = "Impossible de vous écrire en message privé. Ouvrez vos MP pour ce bot, puis réessayez."
	msgDaysInvalid   = "Nombre de jours invalide (1 à 365)."
)

// onInteraction accuse réception puis traite la commande à part.
//
// discordgo exécute les gestionnaires dans la boucle d'événements de la
// passerelle : traiter la commande sur place, avec ses appels réseau, y
// bloquerait la réception des événements suivants.
func (b *Bot) onInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	// Une interaction reçue en message privé nous donne gratuitement
	// l'identifiant du salon : autant le mémoriser plutôt que d'en ouvrir un.
	if i.Context == discordgo.InteractionContextBotDM && i.ChannelID != "" {
		if u := invoker(i.Interaction); u != nil {
			ctx, cancel := context.WithTimeout(b.baseContext(), 5*time.Second)
			if err := b.store.SetDMChannel(ctx, u.ID, i.ChannelID); err != nil {
				b.log.Warn("mémorisation du salon privé impossible", "utilisateur", u.ID, "erreur", err)
			}
			cancel()
		}
	}

	go b.handleCommand(i)
}

func (b *Bot) handleCommand(i *discordgo.InteractionCreate) {
	user := invoker(i.Interaction)
	if user == nil {
		b.log.Warn("interaction sans utilisateur identifiable")
		return
	}

	data := i.ApplicationCommandData()
	log := b.log.With("commande", data.Name, "utilisateur", user.ID)

	ctx, cancel := context.WithTimeout(b.baseContext(), interactionTimeout)
	defer cancel()

	// Accuser réception tout de suite : Discord ferme la fenêtre de réponse au
	// bout de trois secondes, et /follow comme /recap interrogent AniList.
	if err := b.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
	}, discordgo.WithContext(ctx)); err != nil {
		log.Error("accusé de réception impossible", "erreur", err)
		return
	}

	var (
		content string
		err     error
	)
	switch data.Name {
	case cmdFollow:
		content, err = b.handleFollow(ctx, log, user.ID, data)
	case cmdUnfollow:
		content, err = b.handleUnfollow(ctx, user.ID)
	case cmdRecap:
		content, err = b.handleRecap(ctx, log, user.ID, data)
	default:
		content = "Commande non trouvée."
	}
	if err != nil {
		log.Error("échec de la commande", "erreur", err)
		content = msgInternal
	}

	if err := b.edit(ctx, i.Interaction, content); err != nil {
		log.Error("réponse impossible", "erreur", err)
	}
}

func (b *Bot) handleFollow(ctx context.Context, log *slog.Logger, userID string, data discordgo.ApplicationCommandInteractionData) (string, error) {
	opts := optionMap(data.Options)
	username := trimmedString(opts[optUsername])
	rawMask := trimmedString(opts[optMask])

	m, err := mask.Parse(rawMask)
	if err != nil {
		return msgMaskInvalid, nil
	}
	if username == "" {
		return msgUserUnknown, nil
	}

	exists, err := b.anilist.UserExists(ctx, username)
	if err != nil {
		log.Error("vérification du pseudo AniList impossible", "pseudo", username, "erreur", err)
		return msgUserCheckFail, nil
	}
	if !exists {
		return msgUserUnknown, nil
	}

	current := mask.Freqs{}
	existing, err := b.store.Get(ctx, userID)
	switch {
	case err == nil:
		current = existing.Freqs
	case errors.Is(err, store.ErrNotFound):
		// Nouvel abonné : '*' vaut alors « désactivé ».
	default:
		return "", err
	}

	freqs := m.Apply(current)
	if err := b.store.Upsert(ctx, userID, username, freqs); err != nil {
		return "", err
	}
	log.Info("abonnement enregistré", "pseudo", username,
		"mensuel", freqs.Monthly, "annuel", freqs.Yearly)

	msg := fmt.Sprintf("Abonnement enregistré pour **%s** — récap mensuel : %s, récap annuel : %s.",
		username, onOff(freqs.Monthly), onOff(freqs.Yearly))
	if !freqs.Monthly && !freqs.Yearly {
		msg += "\nAucun récapitulatif automatique ne vous sera envoyé. Utilisez `/unfollow` pour supprimer complètement votre abonnement."
		return msg, nil
	}

	if !b.cfg.SendRecapOnFollow {
		return msg, nil
	}

	// Récapitulatif immédiat pour chaque fréquence activée. Le bot JS visait ce
	// comportement mais l'appel échouait en silence, la fonction n'étant pas
	// exportée par son module.
	now := time.Now().In(b.cfg.Location)
	var periods []period.Period
	if freqs.Monthly {
		periods = append(periods, period.PreviousMonth(now, b.cfg.Location))
	}
	if freqs.Yearly {
		periods = append(periods, period.PreviousYear(now, b.cfg.Location))
	}

	var refused bool
	for _, p := range periods {
		if err := b.reports.Send(ctx, userID, username, p); err != nil {
			if errors.Is(err, ErrDMRefused) {
				refused = true
			}
			log.Error("récapitulatif initial non envoyé", "période", p.Key, "erreur", err)
		}
	}
	if refused {
		return msg + "\n" + msgDMRefused, nil
	}
	return msg + "\nUn récapitulatif vient de vous être envoyé en message privé.", nil
}

func (b *Bot) handleUnfollow(ctx context.Context, userID string) (string, error) {
	removed, err := b.store.Remove(ctx, userID)
	if err != nil {
		return "", err
	}
	if !removed {
		return msgNotFollowed, nil
	}
	b.log.Info("abonnement supprimé", "utilisateur", userID)
	return msgUnfollowed, nil
}

func (b *Bot) handleRecap(ctx context.Context, log *slog.Logger, userID string, data discordgo.ApplicationCommandInteractionData) (string, error) {
	opts := optionMap(data.Options)

	days := 0
	if o, ok := opts[optDays]; ok {
		days = int(o.IntValue())
	}
	// Discord applique déjà les bornes déclarées sur l'option ; la vérification
	// reste utile en défense.
	if days < minRecapDays || days > maxRecapDays {
		return msgDaysInvalid, nil
	}

	username := trimmedString(opts[optUsername])
	if username == "" {
		follower, err := b.store.Get(ctx, userID)
		if errors.Is(err, store.ErrNotFound) {
			return msgNoAccount, nil
		}
		if err != nil {
			return "", err
		}
		username = follower.AniListUsername
	}

	p := period.LastDays(time.Now(), b.cfg.Location, days)
	if err := b.reports.Send(ctx, userID, username, p); err != nil {
		if errors.Is(err, ErrDMRefused) {
			return msgDMRefused, nil
		}
		log.Error("récapitulatif non envoyé", "pseudo", username, "jours", days, "erreur", err)
		return "Impossible de produire le récapitulatif pour le moment. Réessayez plus tard.", nil
	}
	return fmt.Sprintf("Récapitulatif des %d derniers jours envoyé en message privé.", days), nil
}

// edit remplace l'accusé de réception par la réponse définitive.
func (b *Bot) edit(ctx context.Context, i *discordgo.Interaction, content string) error {
	_, err := b.session.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: &content},
		discordgo.WithContext(ctx))
	return err
}

// invoker renvoie l'auteur de l'interaction.
//
// En message privé, l'utilisateur est dans Interaction.User ; sur un serveur,
// il est dans Interaction.Member. discordgo ne normalise pas les deux, à la
// différence de discord.js.
func invoker(i *discordgo.Interaction) *discordgo.User {
	if i.User != nil {
		return i.User
	}
	if i.Member != nil {
		return i.Member.User
	}
	return nil
}

func optionMap(opts []*discordgo.ApplicationCommandInteractionDataOption) map[string]*discordgo.ApplicationCommandInteractionDataOption {
	m := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(opts))
	for _, o := range opts {
		m[o.Name] = o
	}
	return m
}

func trimmedString(o *discordgo.ApplicationCommandInteractionDataOption) string {
	if o == nil {
		return ""
	}
	return strings.TrimSpace(o.StringValue())
}

func onOff(v bool) string {
	if v {
		return "activé"
	}
	return "désactivé"
}
