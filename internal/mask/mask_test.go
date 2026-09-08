package mask

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want Mask
	}{
		{"*0", Mask{Monthly: Keep, Yearly: Unset}},
		{"00", Mask{Monthly: Unset, Yearly: Unset}},
		{"**", Mask{Monthly: Keep, Yearly: Keep}},
		{"11", Mask{Monthly: Set, Yearly: Set}},
		{"10", Mask{Monthly: Set, Yearly: Unset}},
		{"*1", Mask{Monthly: Keep, Yearly: Set}},
	}
	for _, tc := range tests {
		got, err := Parse(tc.in)
		if err != nil {
			t.Fatalf("Parse(%q) : erreur inattendue %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("Parse(%q) = %+v, attendu %+v", tc.in, got, tc.want)
		}
		if got.String() != tc.in {
			t.Errorf("Parse(%q).String() = %q", tc.in, got.String())
		}
	}
}

func TestParseInvalid(t *testing.T) {
	for _, in := range []string{"", "1", "111", "2*", "ab", "1 ", " 1", "1é", "**1"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) aurait dû échouer", in)
		}
	}
}

func TestApply(t *testing.T) {
	tests := []struct {
		name    string
		mask    string
		current Freqs
		want    Freqs
	}{
		{
			name:    "l'étoile conserve la valeur courante",
			mask:    "*0",
			current: Freqs{Monthly: true, Yearly: true},
			want:    Freqs{Monthly: true, Yearly: false},
		},
		{
			name:    "nouvel utilisateur, tout en étoile",
			mask:    "**",
			current: Freqs{},
			want:    Freqs{},
		},
		{
			// Régression : le bot JS fusionnait avec un OU logique, un abonnement
			// actif ne pouvait donc jamais être désactivé par le masque.
			name:    "zéro désactive un abonnement déjà actif",
			mask:    "00",
			current: Freqs{Monthly: true, Yearly: true},
			want:    Freqs{Monthly: false, Yearly: false},
		},
		{
			name:    "un active les deux",
			mask:    "11",
			current: Freqs{},
			want:    Freqs{Monthly: true, Yearly: true},
		},
		{
			name:    "daily reste toujours désactivé",
			mask:    "11",
			current: Freqs{Daily: true},
			want:    Freqs{Monthly: true, Yearly: true},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Parse(tc.mask)
			if err != nil {
				t.Fatalf("Parse(%q) : %v", tc.mask, err)
			}
			if got := m.Apply(tc.current); got != tc.want {
				t.Errorf("Apply() = %+v, attendu %+v", got, tc.want)
			}
		})
	}
}
