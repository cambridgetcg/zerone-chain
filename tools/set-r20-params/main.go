package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "set-r20-params: disabled (old params removed in training data protocol pivot R36-5)")
	os.Exit(1)
}
