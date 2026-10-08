# Domain glossary

Existing product terms are defined in [PLAN.md, The model](PLAN.md#3-the-model).
The terms below belong to the proposed guided-commit workflow, not released behavior.

## Active item

The work item explicitly selected as the context for an ongoing workflow. An item's doing
status says work is in progress; it does not alone identify the active item for a caller.

## Commit plan

The declared commit intents for a work item. A guided commit selects one of those intents
rather than inventing its own commit metadata from the changes it contains.

## Commit part

A named intent within a commit plan, carrying its commit type, subject and applicable footer
metadata. A part is not a ledger group or project phase. It may be used for several compliant
commits before completion; using it again after completion requires an approved amendment
or a new part.
