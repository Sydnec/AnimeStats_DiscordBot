package bot

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestCommandDefinitions(t *testing.T) {
	defs := commandDefinitions()
	if len(defs) != 3 {
		t.Fatalf("%d commandes définies, attendu 3", len(defs))
	}

	byName := map[string]*discordgo.ApplicationCommand{}
	for _, d := range defs {
		byName[d.Name] = d
	}

	for _, name := range []string{cmdFollow, cmdUnfollow, cmdRecap} {
		cmd, ok := byName[name]
		if !ok {
			t.Fatalf("commande %q absente", name)
		}
		if cmd.Description == "" {
			t.Errorf("commande %q sans description", name)
		}
		// Sans ces deux champs, Discord n'autorise l'installation que sur un
		// serveur, alors que le bot est prévu pour un compte utilisateur.
		if cmd.IntegrationTypes == nil || len(*cmd.IntegrationTypes) != 2 {
			t.Errorf("commande %q : IntegrationTypes = %v", name, cmd.IntegrationTypes)
		}
		if cmd.Contexts == nil || len(*cmd.Contexts) != 3 {
			t.Errorf("commande %q : Contexts = %v", name, cmd.Contexts)
		}
	}

	if !containsIntegration(byName[cmdFollow].IntegrationTypes, discordgo.ApplicationIntegrationUserInstall) {
		t.Error("l'installation sur un compte utilisateur devrait être autorisée")
	}

	follow := byName[cmdFollow]
	if len(follow.Options) != 2 {
		t.Fatalf("/follow a %d options, attendu 2", len(follow.Options))
	}
	for _, o := range follow.Options {
		if !o.Required {
			t.Errorf("/follow : l'option %q devrait être obligatoire", o.Name)
		}
	}

	if len(byName[cmdUnfollow].Options) != 0 {
		t.Error("/unfollow ne devrait pas avoir d'option")
	}

	recap := byName[cmdRecap]
	if len(recap.Options) != 2 {
		t.Fatalf("/recap a %d options, attendu 2", len(recap.Options))
	}
	days := recap.Options[0]
	if days.Name != optDays || !days.Required {
		t.Errorf("/recap : première option = %+v", days)
	}
	if days.MinValue == nil || *days.MinValue != minRecapDays {
		t.Errorf("/recap : MinValue = %v, attendu %d", days.MinValue, minRecapDays)
	}
	if days.MaxValue != maxRecapDays {
		t.Errorf("/recap : MaxValue = %v, attendu %d", days.MaxValue, maxRecapDays)
	}
	if recap.Options[1].Name != optUsername || recap.Options[1].Required {
		t.Errorf("/recap : seconde option = %+v", recap.Options[1])
	}
}

func containsIntegration(types *[]discordgo.ApplicationIntegrationType, want discordgo.ApplicationIntegrationType) bool {
	if types == nil {
		return false
	}
	for _, t := range *types {
		if t == want {
			return true
		}
	}
	return false
}
