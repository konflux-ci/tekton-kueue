---
name: brainstorming-workflow
description: >
  Use when in an interactive session and the user requests a new feature, significant
  change, refactor, or architectural decision in tekton-kueue. Provides a structured
  process choice before any changes are made. Skip when dispatched with a complete task.
---

# Brainstorming Workflow

Discipline for interactive sessions involving new features, refactors, performance optimizations, API changes, or other significant changes.

## Context Detection

- **Interactive session** (human in CLI/IDE): follow this workflow.
- **Dispatched with a complete task** (sub-agent, automation, explicit spec): skip entirely and execute.

## First Message

Before making any changes, ask exactly ONE question:

> I can approach this a few ways:
>
> A) Jump straight to making changes
> B) Discuss approaches first, then make changes
> C) Full design process — explore approaches, write up a plan, then execute
>
> Which works for you?

**Always ask this question first**, even when the request sounds urgent. It's a quick check that keeps things on track.

**After asking**, if the human replies with "just do it", gives a direct instruction, or otherwise signals urgency, treat as **A**. But ask first — don't skip the question based on tone or urgency cues in the initial request.

## Path A — Jump to Changes

Proceed directly. All existing conventions still apply (pr-workflow, testing requirements). No additional ceremony.

## Path B — Discuss Approaches

1. **Understand the problem**: what is being changed, why, and any constraints.
2. **Propose 2-3 approaches** with trade-offs (complexity, test coverage impact, performance implications, API compatibility).
3. **Lead with a recommendation** and explain why.
4. Let the human choose, then execute.

tekton-kueue examples where this helps:
- Choosing between extending an existing CEL function vs adding a new one
- Deciding how to restructure webhook mutation logic for a new use case
- Planning how to add a new controller reconciler or modify the existing one
- Evaluating whether a change needs new e2e test scenarios or unit tests are sufficient
- Designing changes to the `mutate` CLI vs webhook behavior
- Adding or modifying Kustomize manifests in `config/`

Ask one question at a time. Prefer multiple choice over open-ended questions.

## Path C — Full Design Process

Everything in Path B, plus:

1. **Write up the plan** — what changes in which files/packages, test strategy, API impact.
2. **Break into ordered steps** with dependencies (e.g., add types first, then controller logic, then tests).
3. Get human approval before executing.

## Key Principles

- **One question at a time.** Never pile up multiple questions in one message.
- **Prefer multiple choice.** Easier for the human to decide quickly.
- **Human decides the process, not the agent.** Respect the chosen path.
- **"Just do it" means just do it.** Don't add process the human didn't ask for.
