// Command worker takes scan jobs, crawls, runs rules and stores results.
// Skeleton only (specs/07-architecture.md, specs/10-roadmap.md M2).
package main

import (
	"log"

	"compliancechecker/internal/rules"
	"compliancechecker/internal/rules/a6"
)

func init() {
	rules.Register(a6.New())
}

func main() {
	log.Printf("worker started with %d registered rules", len(rules.All()))
}
