// Where a CI run's range starts. What `itos ci range` decides — a pull
// request's base, else the provider's start when it is an ancestor of the
// head, else empty (run everything), a failing or silent provider no error —
// is tools/itos/conformance/range.yaml's, for the `command` and `none`
// providers and a `github` one with no repository to ask.
//
// Left here: which run the `github` provider takes from the API's answer,
// which no fixture can reach without a network (the conformance runner serves
// no HTTP). The green run is the newest success in the list, whatever its
// order and however many failed or cancelled runs are newer: the API's own
// `status=success` filter can answer with a run far older than that.
import { strict as assert } from "node:assert";
import { firstGreen } from "../itos/providers.ts";

assert.equal(
	firstGreen([
		{ head_sha: "old", conclusion: "success", created_at: "2000-01-01T08:00:00Z" },
		{ head_sha: "red", conclusion: "failure", created_at: "2000-01-09T14:20:00Z" },
		{ head_sha: "new", conclusion: "success", created_at: "2000-01-09T13:33:00Z" },
		{ head_sha: "gone", conclusion: "cancelled", created_at: "2000-01-09T14:33:00Z" },
		{ head_sha: "busy", conclusion: null, created_at: "2000-01-09T14:35:00Z" },
	]),
	"new",
	"the newest success, not the first listed",
);
assert.equal(firstGreen([{ head_sha: "red", conclusion: "failure" }]), undefined);

console.log("ci range: the github provider takes the newest green run's commit");
