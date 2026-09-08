// Package mask interprète le masque d'abonnement de la commande /follow.
//
// Le masque fait exactement deux caractères, dans l'ordre monthly, yearly :
//
//	'1' active l'abonnement
//	'0' le désactive
//	'*' conserve la valeur en base
//
// Le récap quotidien n'est plus configurable : Freqs.Daily reste toujours false,
// la colonne correspondante étant conservée en base pour compatibilité.
package mask

import (
	"fmt"
	"unicode/utf8"
)

// Tri est l'action demandée pour une fréquence donnée.
type Tri uint8

const (
	// Keep conserve la valeur existante ('*').
	Keep Tri = iota
	// Set active la fréquence ('1').
	Set
	// Unset désactive la fréquence ('0').
	Unset
)

func (t Tri) String() string {
	switch t {
	case Set:
		return "1"
	case Unset:
		return "0"
	default:
		return "*"
	}
}

// Length est le nombre de caractères attendu dans un masque.
const Length = 2

// Mask est un masque analysé.
type Mask struct {
	Monthly Tri
	Yearly  Tri
}

// Freqs représente l'état d'abonnement d'un utilisateur.
type Freqs struct {
	Daily   bool
	Monthly bool
	Yearly  bool
}

// ErrInvalid décrit un masque mal formé. Le message est destiné à être affiché
// tel quel à l'utilisateur dans Discord.
type ErrInvalid struct{ Input string }

func (e *ErrInvalid) Error() string {
	return fmt.Sprintf("masque %q invalide : attendu %d caractères parmi 0, 1 et * (ex: 1*)", e.Input, Length)
}

// Parse analyse un masque de deux caractères.
func Parse(s string) (Mask, error) {
	if utf8.RuneCountInString(s) != Length {
		return Mask{}, &ErrInvalid{Input: s}
	}
	runes := []rune(s)
	monthly, ok := parseTri(runes[0])
	if !ok {
		return Mask{}, &ErrInvalid{Input: s}
	}
	yearly, ok := parseTri(runes[1])
	if !ok {
		return Mask{}, &ErrInvalid{Input: s}
	}
	return Mask{Monthly: monthly, Yearly: yearly}, nil
}

func parseTri(r rune) (Tri, bool) {
	switch r {
	case '1':
		return Set, true
	case '0':
		return Unset, true
	case '*':
		return Keep, true
	default:
		return Keep, false
	}
}

// Apply projette le masque sur l'état courant.
//
// C'est ici que se corrige le bug du bot JS, dont la fusion utilisait un OU
// logique (`existing.freq_monthly || desired.freq_monthly`) : un '0' ne pouvait
// alors jamais désactiver un abonnement déjà actif.
func (m Mask) Apply(current Freqs) Freqs {
	return Freqs{
		Daily:   false,
		Monthly: apply(m.Monthly, current.Monthly),
		Yearly:  apply(m.Yearly, current.Yearly),
	}
}

func apply(t Tri, current bool) bool {
	switch t {
	case Set:
		return true
	case Unset:
		return false
	default:
		return current
	}
}

// String rend le masque sous sa forme textuelle d'origine.
func (m Mask) String() string { return m.Monthly.String() + m.Yearly.String() }
