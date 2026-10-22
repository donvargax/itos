package cli

import "github.com/donvargax/itos/v8/internal/engine/agentrules"

// The rules for agents as status and upgrade find them: AGENTS.md, the block
// this itos writes, and olderBlock, a block between itos's markers other
// than that one.
const (
	agentsFile = agentrules.AgentsFile
	rulesBlock = agentrules.Block
	olderBlock = agentrules.Begin + "\n\n## The rules itos holds this repository to\n\n" + agentrules.End + "\n"
)

// staleRulesLine is what a stale block is said as.
func staleRulesLine(stealth bool) string { return agentrules.StaleLine(stealth) }
