# Feature: Quiz Feedback Learning Loop

## Goal
Create a lightweight feedback loop so learners can report question quality issues, and the quiz system can improve over time using that signal.

## Problem Summary
Current quiz flow generates questions, tracks attempts, and stores results, but there is no structured way for users to flag bad questions (wrong answer key, ambiguity, poor wording, and similar issues). As a result, quality problems are not fed back into future generation.

## High-Level Solution
Add a new feedback capability that captures per-question quality feedback and feeds it into two loops:

1. Online heuristic loop:
Use feedback to influence future quiz generation behavior (ranking, selection, and prompt guidance).

2. Offline dataset loop:
Store feedback in a clean, exportable format for later model or prompt tuning workflows.

## User Experience (MVP)
Primary entry point:
Question history cards in the web Knowledge page.

User action:
Select Flag on a question, choose a reason from a curated list, optionally add a short note, and submit.

Curated reason codes:
- incorrect_answer_key
- ambiguous_prompt
- missing_context
- misleading_choices
- duplicate_question
- typo_or_format
- other

## System Behavior
When feedback is submitted:
- Persist an append-only feedback record linked to class, quiz, question, component, and section provenance.
- Deduplicate retries using a deterministic key.
- Expose aggregate views for analysis by question, component, and section.
- Apply bounded quality penalties and anti-pattern guidance during future quiz generation.

## Data Principles
- Local-first storage (no external services in phase 1).
- Append-only records for traceability.
- Curated taxonomy for consistency.
- Optional short free-text notes for nuance.

## What This Enables
- Faster detection of low-quality questions.
- Safer prompt and selection adjustments based on real user outcomes.
- Training-ready dataset export for future improvement work.
- Better trust in quiz quality over repeated usage.

## Not In Scope (First Pass)
- Direct model fine-tuning pipeline execution.
- Full moderation and reviewer UI.
- Cross-interface parity beyond initial web entry point.

## Success Signals
- Feedback submission rate increases over time.
- Repeat flags for the same issue type decrease.
- Post-feedback correctness and confidence trends improve.
- Duplicate and ambiguous question complaints trend downward.

## Rollout Notes
Ship behind a feature toggle if needed. Start with curated taxonomy and minimal UI friction, then expand to CLI and TUI capture plus richer analytics after signal quality is validated.
