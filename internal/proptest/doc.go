// Package proptest is the property-test tier's shared part (T-310, ADR-0051):
// what every stateful property of the domain uses, so that the next domain
// copies a pattern and not a setup. Only _test.go files import it.
//
// A property here is stateful and model-based: rapid draws a random sequence
// of the domain's operations (rapid.T.Repeat, one action per operation), the
// test runs each on the real code and on a plain model, and checks the
// invariants after every step; a failing sequence is shrunk to the shortest
// one. Input-level properties are fuzzing and belong to neither. A property is
// a test named TestProperty<Domain><What>, run through Check, so that the
// nightly selects them by that prefix. Its model is the least that says what
// the domain promises (for the ID counter, the set of numbers handed out and
// the counter), never a second implementation. A concurrent property runs the
// same operations from several goroutines into a History and asks
// Linearizable whether some order of them, no operation moved past one that
// returned before it began, gives every answer the sequential model gives.
//
// The seed is the tier's rule (ADR-0051: deterministic where a tier gates,
// random where it explores). Check pins rapid's seed to FixedSeed unless the
// run names one (RAPID_SEED or -rapid.seed), so the hooks, each push's CI and
// itos-cc's mutation runs draw the same sequences and a kill is stable. The
// nightly names a fresh seed per run, prints it, and raises RAPID_CHECKS (the
// step in itos.yaml's ci.nightly); a failure prints the failing check's seed
// and writes a fail file under the package's testdata/rapid/<test>/. To keep
// a find as a fixed regression, run the test with RAPID_SEED set to the
// printed seed and commit the fail file it writes (git does not ignore it):
// rapid replays every fail file of a test before drawing anything, in every
// tier, whatever the seed. An edit to the property that changes what it draws
// makes rapid ignore an old fail file, saying so; a concurrent property's
// fail file replays its operations, not the schedule that failed them.
//
// A property's cost is its checks times its sequence: Checks sets how many a
// property runs by default (a property over real git or the file system is
// slower than one over a fake), and RAPID_CHECKS, which the nightly raises,
// overrides it. Properties do not run in parallel: the seed and the check
// count are rapid's process-wide flags.
package proptest
