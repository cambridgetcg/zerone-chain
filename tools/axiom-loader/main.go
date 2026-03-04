package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "axiom-loader: disabled (axioms removed in training data protocol pivot R36-5)")
	os.Exit(1)
}
