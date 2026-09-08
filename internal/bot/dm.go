package bot

import (
	"context"
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// Codes d'erreur renvoyés par Discord et traités spécifiquement.
const (
	// errCannotSendToUser : l'utilisateur a bloqué l'application.
	errCannotSendToUser = 50007
	// errCannotSendToUserRestricted : messages privés fermés, ou aucune
	// autorisation commune permettant l'envoi.
	errCannotSendToUserRestricted = 50278
	// errUnknownChannel : le salon mémorisé n'existe plus.
	errUnknownChannel = 10003
)

// ErrDMRefused signale un refus durable : réessayer n'y changera rien.
var ErrDMRefused = errors.New("l'utilisateur n'accepte pas les messages privés du bot")

// SendDM envoie un message privé, en réutilisant le salon déjà connu.
//
// Ouvrir un salon privé est une requête limitée à part par Discord et les
// ouvertures en rafale déclenchent ses garde-fous anti-spam ; l'identifiant est
// donc mémorisé en base après la première ouverture.
func (b *Bot) SendDM(ctx context.Context, userID, content string) error {
	channelID, known, err := b.store.DMChannel(ctx, userID)
	if err != nil {
		b.log.Warn("cache de salon privé illisible", "utilisateur", userID, "erreur", err)
		known = false
	}

	if known {
		err := b.sendTo(ctx, channelID, content)
		if err == nil {
			return nil
		}
		if !isDiscordError(err, errUnknownChannel) {
			return err
		}
		// Le salon mémorisé a disparu : on l'oublie et on en rouvre un.
		b.log.Info("salon privé obsolète, réouverture", "utilisateur", userID)
		if err := b.store.ForgetDMChannel(ctx, userID); err != nil {
			b.log.Warn("oubli du salon privé impossible", "utilisateur", userID, "erreur", err)
		}
	}

	channel, err := b.session.UserChannelCreate(userID, discordgo.WithContext(ctx))
	if err != nil {
		if isDiscordError(err, errCannotSendToUser, errCannotSendToUserRestricted) {
			return fmt.Errorf("%w : %v", ErrDMRefused, err)
		}
		return fmt.Errorf("ouverture du salon privé de %s : %w", userID, err)
	}
	if err := b.store.SetDMChannel(ctx, userID, channel.ID); err != nil {
		b.log.Warn("mémorisation du salon privé impossible", "utilisateur", userID, "erreur", err)
	}

	return b.sendTo(ctx, channel.ID, content)
}

func (b *Bot) sendTo(ctx context.Context, channelID, content string) error {
	_, err := b.session.ChannelMessageSend(channelID, content, discordgo.WithContext(ctx))
	if err == nil {
		return nil
	}
	if isDiscordError(err, errCannotSendToUser, errCannotSendToUserRestricted) {
		return fmt.Errorf("%w : %v", ErrDMRefused, err)
	}
	return err
}

// isDiscordError indique si l'erreur porte l'un des codes Discord donnés.
func isDiscordError(err error, codes ...int) bool {
	var rest *discordgo.RESTError
	if !errors.As(err, &rest) || rest.Message == nil {
		return false
	}
	for _, c := range codes {
		if rest.Message.Code == c {
			return true
		}
	}
	return false
}
