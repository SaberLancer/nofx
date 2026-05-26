// One-off / maintenance tool: sync reduce, exit, and decision_process prompt
// sections on all AI strategies to current defaults (PnL% rules included).
//
// Usage (from repo root):
//
//	go run ./tools/sync-strategy-prompts
//	go run ./tools/sync-strategy-prompts -db data/data.db
package main

import (
	"flag"
	"fmt"
	"os"

	"nofx/store"
)

func main() {
	dbPath := flag.String("db", "data/data.db", "path to SQLite database")
	flag.Parse()

	st, err := store.New(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database: %v\n", err)
		os.Exit(1)
	}

	result, err := st.Strategy().SyncPromptSectionsFromDefaults()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Total strategies: %d\n", result.Total)
	fmt.Printf("Updated (AI):     %d\n", result.Updated)
	fmt.Printf("Skipped (grid):   %d\n", result.Skipped)
	if len(result.UpdatedID) > 0 {
		fmt.Println("Updated IDs:")
		for _, id := range result.UpdatedID {
			fmt.Printf("  - %s\n", id)
		}
	}
	if len(result.Errors) > 0 {
		fmt.Println("Errors:")
		for _, e := range result.Errors {
			fmt.Printf("  - %s\n", e)
		}
		os.Exit(1)
	}
}
