package bot

import "github.com/bwmarrin/discordgo"

const (
	cmdFollow   = "follow"
	cmdUnfollow = "unfollow"
	cmdRecap    = "recap"

	optUsername = "username"
	optMask     = "mask"
	optDays     = "days"

	// Bornes du nombre de jours acceptées par /recap.
	minRecapDays = 1
	maxRecapDays = 365
)

// commandDefinitions décrit les trois commandes du bot.
//
// IntegrationTypes et Contexts sont indispensables : sans eux, Discord
// n'autorise l'installation que sur un serveur, alors que ce bot est prévu
// pour être installé sur un compte utilisateur. Le bot JS ne les déclarait
// pas, ce qui contredisait son propre guide d'installation.
func commandDefinitions() []*discordgo.ApplicationCommand {
	integrations := []discordgo.ApplicationIntegrationType{
		discordgo.ApplicationIntegrationGuildInstall,
		discordgo.ApplicationIntegrationUserInstall,
	}
	contexts := []discordgo.InteractionContextType{
		discordgo.InteractionContextGuild,
		discordgo.InteractionContextBotDM,
		discordgo.InteractionContextPrivateChannel,
	}
	minDays := float64(minRecapDays)

	return []*discordgo.ApplicationCommand{
		{
			Name:             cmdFollow,
			Description:      "Suivre vos stats AniList en MP",
			IntegrationTypes: &integrations,
			Contexts:         &contexts,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        optUsername,
					Description: "Votre pseudo AniList",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        optMask,
					Description: "Masque 2 caractères 'my' (monthly,yearly) où 1=activer, 0=désactiver, *=conserver, ex: 1*",
					Required:    true,
				},
			},
		},
		{
			Name:             cmdUnfollow,
			Description:      "Arrêter de suivre vos stats",
			IntegrationTypes: &integrations,
			Contexts:         &contexts,
		},
		{
			Name:             cmdRecap,
			Description:      "Demander un récap des X derniers jours",
			IntegrationTypes: &integrations,
			Contexts:         &contexts,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        optDays,
					Description: "Nombre de jours à couvrir (1 à 365)",
					Required:    true,
					MinValue:    &minDays,
					MaxValue:    maxRecapDays,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        optUsername,
					Description: "Pseudo AniList (optionnel, sinon celui enregistré)",
				},
			},
		},
	}
}
