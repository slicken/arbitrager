package utils

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Round helper for RoundPlus
func Round(f float64) float64 {
	return math.Floor(f + .5)
}

// // RoundPlus sets decimals by precision
// func RoundPlus(f float64, precision int) float64 {
// 	shift := math.Pow(10, float64(precision))
// 	return Round(f*shift) / shift
// }

// RoundPlus sets decimals by precision
func RoundPlus(f float64, precision int) float64 {
	shift := math.Pow(10, float64(precision))
	return math.Floor(f*shift+.5) / shift
}

// CountDecimal counts decimals
func CountDecimal(v float64) int {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	i := strings.IndexByte(s, '.')
	if i > -1 {
		return len(s) - i - 1
	}
	return 0
}

func TypeName(v interface{}) string {
	return fmt.Sprintf("%T", v)[6:]
}
