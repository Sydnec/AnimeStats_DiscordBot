package bot

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

// TestInvoker couvre la différence entre message privé et serveur : discordgo
// ne normalise pas l'auteur d'une interaction, contrairement à discord.js.
func TestInvoker(t *testing.T) {
	dmUser := &discordgo.User{ID: "111"}
	guildUser := &discordgo.User{ID: "222"}

	tests := []struct {
		name string
		in   *discordgo.Interaction
		want string
	}{
		{"message privé", &discordgo.Interaction{User: dmUser}, "111"},
		{"serveur", &discordgo.Interaction{Member: &discordgo.Member{User: guildUser}}, "222"},
		{"aucun des deux", &discordgo.Interaction{}, ""},
		{"membre sans utilisateur", &discordgo.Interaction{Member: &discordgo.Member{}}, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := invoker(tc.in)
			if tc.want == "" {
				if got != nil {
					t.Errorf("invoker = %+v, attendu nil", got)
				}
				return
			}
			if got == nil || got.ID != tc.want {
				t.Errorf("invoker = %+v, attendu l'identifiant %s", got, tc.want)
			}
		})
	}
}

func TestOptionMap(t *testing.T) {
	opts := []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: optUsername, Type: discordgo.ApplicationCommandOptionString, Value: "  Sydnec  "},
		{Name: optMask, Type: discordgo.ApplicationCommandOptionString, Value: "1*"},
	}
	m := optionMap(opts)
	if len(m) != 2 {
		t.Fatalf("%d options indexées, attendu 2", len(m))
	}
	if got := trimmedString(m[optUsername]); got != "Sydnec" {
		t.Errorf("trimmedString = %q, attendu %q", got, "Sydnec")
	}
	if got := trimmedString(m["absent"]); got != "" {
		t.Errorf("une option absente devrait donner une chaîne vide, obtenu %q", got)
	}
}

func TestIsDiscordError(t *testing.T) {
	err := &discordgo.RESTError{
		Message: &discordgo.APIErrorMessage{Code: errCannotSendToUser, Message: "Cannot send messages to this user"},
	}
	if !isDiscordError(err, errCannotSendToUser, errCannotSendToUserRestricted) {
		t.Error("le code 50007 aurait dû être reconnu")
	}
	if isDiscordError(err, errUnknownChannel) {
		t.Error("le code 10003 ne devrait pas correspondre")
	}
	if isDiscordError(nil, errUnknownChannel) {
		t.Error("une erreur nulle ne devrait correspondre à aucun code")
	}
}

func TestOnOff(t *testing.T) {
	if onOff(true) != "activé" || onOff(false) != "désactivé" {
		t.Error("libellés d'état inattendus")
	}
}
