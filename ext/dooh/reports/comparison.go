package reports

import (
	"fmt"
	"math"
	"strings"
)

type comparison map[string][3]float64

func compare(a, b map[string]float64) comparison {
	c := make(map[string][3]float64)
	for _, field := range Values {
		a := a[field]
		b := b[field]
		if a == 0 && b != 0 {
			c[field] = [3]float64{a, b, 1}
		} else if a != 0 {
			c[field] = [3]float64{a, b, b/a - 1}
		}
	}

	return c
}

func (c comparison) breaks(threshold float64) bool {
	for _, entry := range c {
		if math.Abs(entry[2]) > threshold {
			return true
		}
	}

	return false
}

func (c comparison) String() string {
	var b strings.Builder
	for field, entry := range c {
		diff := entry[2]
		sign := ""
		if diff > 0 {
			sign = "+"
		} else if diff < 0 {
			sign = "-"
		} else {
			continue
		}

		b.WriteString(fmt.Sprintf("%s: %.0f / %.0f / %s%.3f%s\n", field, entry[0], entry[1], sign, 100*math.Abs(diff), "%"))
	}

	return strings.Trim(b.String(), "\n")
}
