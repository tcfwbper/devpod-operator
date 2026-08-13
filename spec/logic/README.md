# Logic Specification Style Guide

This document defines the conventions for writing logic spec files and provides language-agnostic guidance. But you should follow the target programming conventions when you write logic spec files.

---

## Purpose

A logic specification describes **what** a unit of logic does — its contract with the outside world — without prescribing **how** it is implemented. It serves as the single source of truth for behavior expectations.

---

## Scope

Each specification file covers exactly one logical unit. A logical unit is any discrete, nameable piece of behavior:

- a function / procedure
- a module / service
- an algorithm
- a business rule or policy

Do **not** mix multiple unrelated units in one file.

---

## File Structure

Every specification file must contain the following sections, in order:

```
# <Unit Name>

## Overview
## Boundaries
## Dependencies
## Behavior
## Inputs
## Outputs
## Invariants
## Edge Cases
## Related
```

Omit a section only if it genuinely does not apply.

The added `Boundaries` and `Dependencies` sections are not optional for multi-component logic. If the unit has collaborators, lifecycle hand-offs, persistence, cleanup, concurrency, timers, signals, or construction constraints, both sections must be present.

---

## Contract Strength

The goal is not only to describe behavior, but also to prevent incorrect ownership and cross-layer implementations during code generation.

Each logic spec must make the following items explicit whenever they apply:

1. **Owner of the behavior** — which unit is responsible for performing the action.
2. **Non-owner units** — which nearby units must not perform that action.
3. **Interaction surface** — which collaborator methods or callbacks this unit is allowed to invoke.
4. **Forbidden interactions** — which collaborators, resources, or lifecycle steps this unit must not manage directly.
5. **Construction rules** — whether the implementation must use an existing constructor / factory / adapter instead of direct field or struct initialization.
6. **Failure authority** — which unit is allowed to convert a condition into a terminal failure, cleanup, retry, or human-facing error.

If any of these are left implicit, an AI coding agent will often guess incorrectly at the abstraction boundary.

---

## Section Conventions

### Overview

One to three sentences. State:

1. What the unit does (its responsibility).
2. What it does **not** do (scope boundary, if non-obvious).

For orchestration or lifecycle code, explicitly name the next owner for adjacent responsibilities. Example: *"Creates the session entity but does not create the runtime socket; socket lifecycle is owned by Runtime."*

### Boundaries

Describe responsibility ownership and explicit non-responsibilities.

- Use short bullets.
- Include at least one positive ownership statement and one negative boundary statement when the unit collaborates with other units.
- Prefer strong wording: *"Owns ..."*, *"Delegates ... to ..."*, *"Must not ..."*.
- If the unit participates in a larger lifecycle, name the hand-off point.

Recommended format:

```markdown
## Boundaries

- Owns: <behavior or resource this unit is responsible for>
- Delegates: <behavior delegated to another unit>
- Must not: <forbidden behavior 1>
- Must not: <forbidden behavior 2>
```

### Dependencies

List collaborators and allowed interaction surfaces. This is where you constrain how code generation may wire the unit.

Use a table when the unit has more than one collaborator:

| Collaborator | Role | Allowed Interaction | Forbidden Interaction |
|---|---|---|---|
| `FooStore` | Persistence dependency | `Write()` | Must not construct sockets |

Also include construction constraints here when applicable:

- Name the required constructor / factory / adapter.
- State whether direct struct literals, field assignment, or bypassing adapters is forbidden.
- State whether a dependency is internally constructed, externally injected, or created only after some prerequisite identifier exists.

### Behavior

Describe the core logic as a numbered list of declarative statements.

- Use present tense: *"Returns the largest element."*
- One statement per observable behavior.
- Avoid implementation language (no pseudo-code unless unavoidable).
- Order from the happy path to exceptional paths.

Additional requirements for generation-oriented specs:

- Put cross-component ordering requirements here when order is externally observable or architecturally important.
- State checkpoint behavior explicitly around timeouts, retries, cleanup, notifications, and partial failures.
- If a behavior is delegated, say so directly instead of restating the delegate's internal behavior.
- Do not assign responsibility to this unit for actions owned by another unit.

### Inputs

A table or list. For each input, specify:

| Field | Description |
|-------|-------------|
| Name | Identifier used in the spec |
| Type | Logical type (e.g., integer, list, boolean) — not a language type |
| Constraints | Valid range, allowed values, nullability |
| Required? | Yes / No / Conditional |

### Outputs

Same format as Inputs. Include error / failure outputs.

If the unit can fail before a value is constructed versus after partial construction, state those cases separately.

### Invariants

Conditions that must hold **at all times**, regardless of input:

- State what the unit never violates.
- Use *"Must …"* / *"Must not …"* phrasing.
- Keep each invariant independently testable.

Include boundary invariants whenever applicable:

- ownership invariants
- constructor / factory usage invariants
- persistence or cleanup authority invariants
- thread-safety or signal-handling invariants
- "first error wins" or similar failure-authority invariants

### Edge Cases

List inputs or states that require special handling. For each:

```
- Condition: <what triggers this case>
  Expected: <what the unit must do>
```

Edge cases must stay within the unit's ownership. If an edge case is primarily handled by a neighboring unit, either move it to that unit's spec or phrase this unit's obligation as delegation / propagation only.

### Related

Links to related specs, external references, or upstream policies. Use relative paths for workspace files.

Always link the most relevant neighboring owner when the unit explicitly does **not** own an adjacent responsibility.

---

## Writing Rules for AI Generation

Use these rules to make the spec safer for code generation:

1. Do not rely on "obvious" ownership. Write it down.
2. Do not let `Behavior` imply resource ownership that `Boundaries` denies.
3. Do not let `Edge Cases` silently expand the unit's scope.
4. If the codebase already has a constructor / factory / adapter that protects invariants, the spec must say that the implementation uses it.
5. If another unit owns cleanup, retries, socket lifecycle, signal handling, persistence, or human-facing reporting, say that this unit must only notify / return / delegate.
6. If testing the unit requires a seam such as an injected timer, callback, exit function, or factory, mention that seam in `Dependencies` rather than forcing downstream units to absorb the behavior.
