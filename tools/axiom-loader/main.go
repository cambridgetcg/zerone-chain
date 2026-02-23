package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "validate":
		err = runValidate(args)
	case "inject":
		err = runInject(args)
	case "stats":
		err = runStats(args)
	case "edges":
		err = runEdges(args)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: axiom-loader <command> [args]

Commands:
  validate <axioms.json>                    Validate axiom DAG
  inject   <axioms.json> <genesis.json>     Inject axioms into genesis
  stats    <axioms.json>                    Print axiom statistics
  edges    <axioms.json> [-o output.csv]    Export dependency edges as CSV
`)
}

func runValidate(args []string) error { return fmt.Errorf("not implemented") }
func runInject(args []string) error   { return fmt.Errorf("not implemented") }
func runStats(args []string) error    { return fmt.Errorf("not implemented") }
func runEdges(args []string) error    { return fmt.Errorf("not implemented") }
