package utils

import (
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Round helper for RoundPlus
func Round(f float64) float64 {
	return math.Floor(f + .5)
}

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

// FloatImitate imitates decimals of candidate
func FloatImitate(f, candidate float64) float64 {
	shift := math.Pow(10, float64(CountDecimal(candidate)))
	return math.Floor(f*shift) / shift
}

func TypeName(v interface{}) string {
	return fmt.Sprintf("%T", v)[6:]
}

// LogToFile ...
func LogToFile(tag string) {
	if tag != "" {
		tag = tag + "_"
	}
	logName := tag + time.Now().Format("20060102") + ".log"
	logFile, err := os.Create(logName)
	if err != nil {
		log.Fatalf("could not create %q: %v", logFile.Name(), err)
	}
	log.SetOutput(io.MultiWriter(os.Stderr, logFile))
	log.Printf("logging to %q\n", logFile.Name())
}

func cls() {
	cls := exec.Command("clear")
	cls.Stdout = os.Stdout
	cls.Run()
}
