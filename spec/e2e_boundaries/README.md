# E2E Boundaries Reference

## Purpose

A logic spec's `Dependencies` table names external collaborators but only describes the *allowed interaction*, not whether that system can safely be touched by an automated end-to-end test. This directory is the single source of truth for that second question: for every real-world external touchpoint used anywhere in `logic/`, is there a safe, repeatable, automated way to exercise it, and if so, how exactly.

Without this catalog, deciding whether and how to write an `e2e` test row requires guessing facts about the outside world that aren't derivable from `logic/` or `CONVENTIONS.md` — whether a system is reachable at all, what state may be assumed to exist, how long a real round-trip takes. Every such fact must be looked up here, never invented.

How a boundary's facts get turned into an actual `e2e` test spec row — including what to do when a scenario would need to cross a boundary marked `no` — is a QA-workflow decision, not a concern of this directory. This directory only states the facts about each boundary; it does not prescribe how any agent must react to them.

## Ownership

This directory is maintained **only** by the QA Analyst role (human-in-the-loop). It lives at the `spec/` root.

## File Layout

One file per boundary: `spec/e2e_boundaries/<mirrored-logic-directory>/<boundary-key>.md`. `<boundary-key>` is a stable, kebab-case identifier — it **is** the filename (no separate key field inside the file). Anything that needs to reference a boundary from elsewhere (a test spec, a discussion) does so by this filename, qualified with its directory when the key alone is ambiguous.

`<mirrored-logic-directory>` is the same directory path a boundary's collaborators live under in `logic/`. For example, collaborators declared in `logic/common/docker/*.md` produce boundary files under `spec/e2e_boundaries/common/docker/`. This correspondence is directory-level only.

Keeping one boundary per file, instead of one large table, means looking something up means finding the one file whose `Maps To Collaborators` section names the collaborator in question — not reading through every boundary ever catalogued.

## Writing Rules

- One file per real-world external touchpoint, not per logic spec and not per collaborator class. If two collaborators in different logic specs both talk to the same SMB share, they share one file.
- A boundary key is stable once any file references it. If the underlying system's shape changes enough that the old facts no longer apply, add a new key rather than repurposing the old one.
- Name exact commands, tools, environment variables, or account/project identifiers for whatever this boundary does fix. "A sandbox account is available" is not usable; "use the sandbox project `<name>`, credentials in `MEND_E2E_API_KEY` / `MEND_E2E_USER_KEY`" is. This does not mean inventing one fixed answer for a choice that belongs to the calling test — see the delegation rule below.
- Never record a literal secret value — only the name of the environment variable or secret-manager reference that holds it.
- **Delegate a genuinely open choice to the calling test instead of inventing an answer for it.** Not everything a `yes` boundary needs is a fact about the external system — which of several equally-valid sandbox targets to hit, what test data or identifier to operate on, how to namespace one run, are decisions the calling test is free to make per scenario.
- **Scope the boundary to the collaborator's actual contract, not to the branded system behind it.** Look at what the codebase's collaborator is declared to depend on in its own `logic/*.md` `Boundaries`/`Dependencies` sections — if that contract is narrower than the full real-world system (for example, a collaborator that only needs "a filesystem path to read and write" rather than "a real corporate SMB session"), a stand-in that satisfies exactly that narrow contract is a legitimate `yes`, even though the full branded system is out of reach. Conversely, do not let a stand-in's plausibility be used to assert an outcome owned by a *different* collaborator (e.g. a unit that only places a file must not have its e2e test claim that a downstream scan of that file succeeded) — that claim belongs in that other collaborator's own boundary file, and may itself be a `no`.
- A boundary being expensive or slow in its normal/production shape does not make it a `no` by default — check whether a smaller, disposable version of the same real interaction (a tiny sample project, a throwaway sandbox tenant, skipping a wait that only exists to let a backend catch up when the test doesn't need fresh results) still produces a meaningful signal. Reach for `no` when no such reduction exists, not merely when the realistic version is inconvenient.

## File Structure

```markdown
# <boundary-key>

## Maps To Collaborators
- every collaborator name / module path this boundary represents, taken
  verbatim from `logic/*.md` `Dependencies` tables

## System Description
one or two sentences: what the real external system is, and why units touch it

## Automatable
`yes`, or `no — <one-sentence reason>`
```

If `Automatable` is `no`, stop there — the file needs nothing further. A documented `no` is a complete, valid answer.

If `Automatable` is `yes`, continue with:

```markdown
## Setup
how this boundary is brought into a testable state: the exact command, tool,
sandbox account, or disposable resource used; what already exists vs. what
the test creates; where a stand-in's behaviour is grounded (a recorded
fixture, a documented contract) if it isn't the real system; the exact
credential env var(s) and their privilege tier; why running this repeatedly
has no unacceptable real-world effect. Where part of this is a genuinely
open choice rather than a fact about the system, say so explicitly and name
the constraint the calling test's choice must satisfy, instead of picking
one value for it (see Writing Rules)

## Cleanup & Namespacing
(only if Setup creates state that outlives a single test run)
how created resources are identified and disposed of, and whether concurrent
test runs can collide on this boundary. Identification and disposal may
themselves be the calling test's responsibility to carry out — state that
plainly rather than describing a cleanup mechanism this file does not
actually perform

## Timeout Budget
number in seconds — the maximum wall-clock time a test touching
this boundary may take. Allows different timeout budgets for different test scenarios. This file states the number only; which mechanism
enforces it (a test-framework timeout feature, a wrapper script, a CI step
limit) is a project- and language-specific detail decided outside this
directory.

## Non-Goals
(optional) behaviour that must not be asserted even though this boundary is
reachable
```
