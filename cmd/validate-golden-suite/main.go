package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ldamasio/truthmetal/internal/goldensuite"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: validate-golden-suite <suite-directory>")
		os.Exit(2)
	}
	summary, err := goldensuite.ValidateDir(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid golden suite: %v\n", err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(summary); err != nil {
		fmt.Fprintf(os.Stderr, "encode validation summary: %v\n", err)
		os.Exit(1)
	}
}
