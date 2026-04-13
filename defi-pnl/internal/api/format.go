package api

import (
	"fmt"
	"strings"
)

// formatDecimalUS formats v with decimals fractional digits and thousands separators (e.g. 30801997.065 -> "30,801,997.07").
func formatDecimalUS(v float64, decimals int) string {
	s := fmt.Sprintf("%.*f", decimals, v)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	parts := strings.SplitN(s, ".", 2)
	out := addCommas(parts[0])
	if len(parts) == 2 {
		out += "." + parts[1]
	}
	if neg {
		return "-" + out
	}
	return out
}

func addCommas(intDigits string) string {
	n := len(intDigits)
	if n <= 3 {
		return intDigits
	}
	var b strings.Builder
	first := n % 3
	if first == 0 {
		first = 3
	}
	b.WriteString(intDigits[:first])
	for i := first; i < n; i += 3 {
		b.WriteByte(',')
		b.WriteString(intDigits[i : i+3])
	}
	return b.String()
}
