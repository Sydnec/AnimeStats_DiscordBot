package render

import (
	"math"
	"strconv"
)

// Minutes met en forme une durée : « 6h12min », ou « 6h » quand les minutes
// tombent à zéro. Les minutes sont arrondies à l'entier le plus proche.
func Minutes(mins float64) string {
	if math.IsNaN(mins) || mins <= 0 {
		return "0h"
	}
	total := int(math.Round(mins))
	h := total / 60
	m := total % 60
	if m == 0 {
		return strconv.Itoa(h) + "h"
	}
	return strconv.Itoa(h) + "h" + strconv.Itoa(m) + "min"
}
