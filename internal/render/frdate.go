// Package render met en forme les récapitulatifs envoyés en message privé.
package render

import (
	"strconv"
	"time"
)

// Go ne dispose pas d'équivalent d'Intl : les libellés sont donc écrits à la
// main. Les valeurs ci-dessous reproduisent exactement celles que produisait le
// bot JS, relevées sur l'ICU de Node.
//
// monthsAbbr correspond à Intl.DateTimeFormat("fr-FR", {dateStyle: "medium"}).
// Ni « mars », ni « mai », ni « juin », ni « août » ne portent de point : ces
// mois ne sont pas abrégés.
var monthsAbbr = [12]string{
	"janv.", "févr.", "mars", "avr.", "mai", "juin",
	"juil.", "août", "sept.", "oct.", "nov.", "déc.",
}

// monthsLong correspond à toLocaleString("fr-FR", {month: "long", year: "numeric"}).
var monthsLong = [12]string{
	"janvier", "février", "mars", "avril", "mai", "juin",
	"juillet", "août", "septembre", "octobre", "novembre", "décembre",
}

// Date rend une date au format court français : « 12 sept. 2025 ».
// Le quantième n'est pas complété par un zéro, et le premier du mois s'écrit
// « 1 sept. 2025 » et non « 1er ».
func Date(t time.Time) string {
	return strconv.Itoa(t.Day()) + " " + monthsAbbr[t.Month()-1] + " " + strconv.Itoa(t.Year())
}

// MonthYear rend « septembre 2025 ».
func MonthYear(t time.Time) string {
	return monthsLong[t.Month()-1] + " " + strconv.Itoa(t.Year())
}

// DayKey rend une journée « 2025-09-12 » telle que produite par le moteur de
// statistiques. La clé est déjà exprimée dans le fuseau voulu : elle est
// relue telle quelle, sans conversion.
func DayKey(key string) (string, bool) {
	d, err := time.Parse("2006-01-02", key)
	if err != nil {
		return "", false
	}
	return Date(d), true
}
