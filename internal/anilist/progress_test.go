package anilist

import (
	"encoding/json"
	"testing"
)

func TestProgressUnmarshal(t *testing.T) {
	tests := []struct {
		raw       string
		wantValid bool
		wantText  string
	}{
		{`"3"`, true, "3"},
		{`3`, true, "3"},
		{`"1 - 3"`, true, "1 - 3"},
		{`"1 – 3"`, true, "1 – 3"},
		{`null`, false, ""},
		{`3.5`, true, "3.5"},
		{`""`, true, ""},
	}
	for _, tc := range tests {
		var p Progress
		if err := json.Unmarshal([]byte(tc.raw), &p); err != nil {
			t.Fatalf("Unmarshal(%s) : %v", tc.raw, err)
		}
		if p.Valid != tc.wantValid || p.Text != tc.wantText {
			t.Errorf("Unmarshal(%s) = {Valid:%v Text:%q}, attendu {Valid:%v Text:%q}",
				tc.raw, p.Valid, p.Text, tc.wantValid, tc.wantText)
		}
	}
}

func TestProgressRoundTrip(t *testing.T) {
	for _, p := range []Progress{{}, NewProgress("3"), NewProgress("1 - 3")} {
		b, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("Marshal(%+v) : %v", p, err)
		}
		var back Progress
		if err := json.Unmarshal(b, &back); err != nil {
			t.Fatalf("Unmarshal(%s) : %v", b, err)
		}
		if back != p {
			t.Errorf("aller-retour = %+v, attendu %+v", back, p)
		}
	}
}

func TestMediaHelpers(t *testing.T) {
	var nilMedia *Media
	if got := nilMedia.EpisodeDuration(); got != DefaultEpisodeDuration {
		t.Errorf("durée d'un média nil = %d", got)
	}
	if got := nilMedia.PreferredTitle(); got != TitreInconnu {
		t.Errorf("titre d'un média nil = %q", got)
	}
	if got := nilMedia.MediaID(); got != "noid" {
		t.Errorf("clé d'un média nil = %q", got)
	}

	// Le bot JS écrivait `media.duration || 24` : une durée nulle retombe donc
	// sur la valeur par défaut.
	zero := 0
	m := &Media{Duration: &zero}
	if got := m.EpisodeDuration(); got != DefaultEpisodeDuration {
		t.Errorf("durée nulle = %d, attendu %d", got, DefaultEpisodeDuration)
	}

	// Un média sans identifiant partage la clé de repli "noid", comme en JS.
	if got := (&Media{ID: 0}).MediaID(); got != "noid" {
		t.Errorf("clé d'un média sans id = %q", got)
	}

	full := &Media{Title: Title{English: "English only"}}
	if got := full.PreferredTitle(); got != "English only" {
		t.Errorf("titre préféré = %q", got)
	}
	native := &Media{Title: Title{Native: "ネイティブ"}}
	if got := native.PreferredTitle(); got != "ネイティブ" {
		t.Errorf("titre natif = %q", got)
	}
}
