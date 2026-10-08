# Guided item-aware commits

## Status

Design interview, begun 2026-10-08. The user agreed to replace the git shim's transparent
commit/push forwarding with guidance into an item-aware itos workflow. This document is a
draft, not an implementation brief or a released interface. Proposed commands are not
available today. Implementation stays separate from the ongoing slice-102 work.

## Agreed direction

- In managed checkouts, attempts to use git commit or git push should fail with a useful
  explanation and the appropriate itos command. Git inspection commands remain available.
  The message should say the requested operation is disabled, not that all git is absent.
- Itos should know the selected work item through explicit local checkout/worktree context.
  The shim must not guess from an ambiguous list of items marked doing.
- The implementing agent mainly supplies the commit body: what changed and why. Itos
  assembles the Conventional Commit header and the required footers from structured context.
- Recording work, pushing it and closing the item remain distinct operations. A commit does
  not mean done; closure still requires the configured checks and green CI.
- The shim remains a usability guardrail, not a security boundary. Core rules, hooks and CI
  remain responsible for judging commits, including commits made through a direct git path.

## Existing constraints

A task's ledger type and title are useful metadata, but do not describe every commit an item
needs. The red-first test commit, implementation commit and documentation commit can have
different types and path scopes. Feat/fix footers also need scenario selection and upgrade
text; a breaking change needs its reason and the old promises it changes. Missing semantic
metadata must not silently become guessed values or Upgrading: none.

The current shim forwards managed git commit/push to itos and passes other commands through.
It has no active-item association. Current itos commit accepts a complete message and writes
footers from flags. Work done checks that the implementation has already landed, then closes
the registry item; it does not commit or push the implementation.

## Decision: every guided commit selects a declared part

The user chose a required lightweight commit plan. Each named part defines
its type, short imperative subject and applicable footer metadata. The agent selects a part
and supplies its body. Itos validates staged paths against that part and the repository's
existing scopes. If the work needs another part, update the plan rather than infer a different
type or subject from the diff.

For example, a slice might have test steps, implementation and documentation parts. This is
not a mandatory three-commit template: a simple task may need one part, and an item may need
more than one commit within an approved part, as the reuse decision below permits.

Alternatives are a plan with an inferred fallback, or fully inferred headers/footers. Those
reduce specification work but leave the tool making semantic decisions that staged paths
alone cannot establish, such as feat versus fix or the consumer's upgrading instructions.

Every guided commit must use a declared part. Itos refuses missing semantic metadata or
staged paths that do not fit, rather than silently inferring a different commit. The exact
storage and authoring mechanism remains undecided.

## Decision: the coordinator approves the plan and its amendments

The user chose coordinator approval. The coordinator prepares the plan with the specification. An implementing
agent can propose a missing part or an amendment, but the coordinator must approve and record
it before that agent commits against it. Approval is not a claim that a text file proves which
person or agent edited it; enforcement and the trust model still need a design.

Allowing the implementer to append parts or change the plan freely was rejected: it would let
the same agent choose the commit metadata that was meant to constrain its work. The approval
mechanism remains open; this decision does not claim that an editable file enforces authorship.

## Decision: a part may have several commits before completion

The user chose reusable parts. A part is a reusable approved intent until it is completed, not a prediction
of the number of commits the work will need. Every commit still meets the part's fixed type,
subject, footers and path rules, and the normal gates. Completing a part prevents further
commits against it without a coordinator-approved amendment or new part. How completion is
declared and proved remains open.

One commit per part was rejected because every additional checkpoint or correction would
need another approved part. Reuse does not make a checkpoint ready to push or close the item;
the existing gates still judge every commit. Completion declaration and proof remain open.

## Decision: recorded workflow approval first

The current coordinator and implementing sessions can run as the same OS/git identity in
the same checkout. A command named approve or a coordinator role label therefore does not
prove who approved a plan. A stored hash can identify the approved content, but cannot by
itself distinguish an authorized amendment from a self-approved one.

The user chose workflow first. The first version enforces plan structure and the selected
part's constraints, records the approved plan revision and explicit amendments, and describes
coordinator ownership as a reviewed workflow rule. Do not claim a security boundary. A hard
approval boundary needs a trusted approval mechanism the implementing environment cannot
use or modify, such as isolated workers with approval held on the host or verified signed
approvals. That stronger boundary is not a prerequisite for this first guided-workflow version.

## Decision: separate machine-readable plan files

The user chose separate files. ADR-0034 keeps the work registry as an index, not the specification. Use a
separate machine-readable plan file for each item, linked from its registry entry, and have
itos commands write and validate it. This gives tasks, slices and bugs one format without
embedding a new parser inside Gherkin comments. A private/stealth project would keep the
corresponding data beside its existing private itos data rather than publishing it.

The alternatives are inline metadata in each item's existing specification, or a plan object
in the registry. Inline metadata would need consistent reading across task YAML and feature
files; a registry object is convenient to find but changes ADR-0034's separation. Storage
location, filename and schema are not settled yet; the alternatives were not chosen.

## Decision: work take activates local item context

The user chose activation on take. A successful work take also activates that item locally for the checkout.
Provide an explicit selection command for resuming an already-taken item; it does not change
registry ownership or status. Keep context per checkout/worktree rather than in the shared
git common directory, so two worktrees do not overwrite each other's selection. Successful
closure clears that context. A missing, stale or ambiguous context refuses the guided commit
and gives the selection command instead of guessing from doing items.

A separate selection after every take and mandatory item IDs on every commit were not
chosen. The concrete context file and selection command are still to be designed.

## Decision: an explicit final-commit flag declares completion

The user chose a final-commit flag. The last guided commit for a part takes an explicit complete flag. After
its commit succeeds, it records that commit as the author's declaration that the part is
complete. This is not proof that the item has landed or passed CI. Work done still requires
the configured checks and green CI, plus completion declarations for required plan parts.
A failed commit records no completion. Reopening a completed part needs an approved plan
amendment; a passing local hook alone does not automatically declare the work complete.

A separate part-done operation was not chosen as the normal path. Recovery when the flag
was omitted, completion records and optional parts still need a concrete contract.

## Decision: an item-aware work commit command

The user chose work commit. Introduce an item-aware work commit command and have the shim point agents
to it. It takes body input and a declared part, assembles metadata and records completion
when requested. Keep the existing low-level itos commit interface for explicit full-message
use; guides for implementing agents recommend the item-aware command. Keeping that route
does not claim that the guidance cannot be bypassed.

The alternative is replacing itos commit itself with the body-only interface, making its
current full-message and git-argument interface an intentional public API break. This
alternative was not chosen. Exact flags, input modes and preview behavior are not yet settled.

## Decision: optional message preview

The user chose optional preview. Offer a dry-run preview of the assembled message, selected item and
part, approved plan revision and staged paths. It makes no commit and records no completion.
The actual commit resolves and validates the current inputs again, so an earlier preview is
not permission to commit a different staged tree or changed plan. Do not require interactive
confirmation for every automated commit when the metadata was already approved.

The alternative is a mandatory prepare/confirm cycle for every commit. That makes inspection
an explicit workflow step but does not prove a human reviewed it, and it adds an invocation.
The mandatory prepare/confirm cycle was not chosen.

## Decision: guided refusal is opt-in initially

The user chose opt-in first. Introduce guided refusal as an explicit setup choice for the first release,
and require a compatible pinned itos before enabling it. Existing transparent-shim users
would not silently change behavior while this workflow is being tried. A later default
replacement must be an intentional breaking release, not a compatibility fallback.

The alternative is making guided refusal the default replacement immediately in the release
that introduces it; that alternative was not chosen. It changes editor/script
commit behavior and needs the corresponding breaking-change specification and migration.
Release version and implementation scheduling are not decided by this document.

## Remaining branches of the design

Still to resolve: part completion, body input and preview, part selection, required/optional
parts, scenario and breaking-change metadata, staged-path refusal and splitting, state-specific
shim messages, closure flow and migration of existing git-shim users. Concrete paths, schema
and command spelling are proposals until the workflow is settled.

## References

- [Core model and non-goals](../../PLAN.md).
- [Existing repository workflow](../ORCHESTRATING.md).
- [Shim behavior](../../internal/shim/doc.go).
- [Commit and work-done help](../../internal/cli/help.go).
- [The registry is an index](../decisions/0034-the-work-registry-is-an-index-and-a-why-lives-in-the-spec.md).
