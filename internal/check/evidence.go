package check

import "github.com/donvargax/itos/v8/internal/evidence"

// Evidence is the engine that looks up the checks that opt in to evidence
// records by the index tree and records the ones it runs
// (internal/evidence), which a Runner is given and a command that runs
// checks reads the invocation's evidence from. EvidenceInfo is what --json
// says of one check's.
type (
	Evidence     = evidence.Engine
	EvidenceInfo = evidence.Info
)
