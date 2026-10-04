# 1. Defaults follow common conventions, and configuration is the escape hatch

Date: 2026-10-04

## Status

Accepted

## Context

A repository set up with adr-tools' defaults keeps its records in doc/adr with no .adr-dir file, so itos ask record would start a second log in docs/adr, numbered from 1. Should an existing doc/adr holding records win when there is no .adr-dir, as adr-tools' own lookup does (the coordinator's recommendation: compatibility was the point of using its format), or stay with docs/adr unless .adr-dir says otherwise?

Asked as q-8, about p1-adr-tools-default-dir.

## Decision

No (2026-10-04): docs/adr stays the default and a different folder is configured, with a .adr-dir file. Defaults follow the most common convention; configuration is the escape hatch (convention over configuration). An existing doc/adr does not win on its own.

## Consequences

A new default is the most common convention for its setting, and says which one it follows (docs/adr for decision records, as adr-tools and log4brains use it). A key or file that overrides a default is added for the person who needs the escape hatch, never instead of the default. A layout that differs from the convention is configured, never guessed from what the repository holds.
