# Feature: Coverage Intelligence Workflow

## Goal
Make coverage configuration intent-first instead of schema-first, so users can describe what they want in natural language and the system can suggest, validate, and apply coverage weighting safely.

## Problem Summary
Current coverage editing expects users to know internal metadata details such as labels, tags, and source patterns ahead of time.

This causes three major issues:
- Discoverability gap: users do not know what metadata exists after ingestion.
- Fragility gap: manual rules drift as new notes are ingested.
- Trust gap: users cannot easily preview or understand impact before changes affect quiz generation.

## What Coverage Means Today
Coverage influences quiz generation in two ways:
- Score weighting: candidate components are reweighted by group matches (tags and source patterns).
- Prompt guidance: coverage priorities are merged into class profile context for orchestrator planning.

Coverage can also exclude unmatched material when enabled, which can dramatically narrow the candidate pool.

## High-Level Solution
Introduce a feedback-loop workflow that runs after ingestion and uses an agent-backed natural language layer to keep coverage current.

The workflow has three lanes:
1. Automatic analysis lane (post-ingestion)
2. Natural language refinement lane (user intent commands)
3. Safe apply lane (policy checks, preview, and auditability)

## User Experience (Target)
After ingestion completes:
- User sees a "Coverage Insights" summary.
- System proposes suggested groups with confidence and rationale.
- High-confidence suggestions may auto-apply under guardrails.
- User can review diffs, approve/reject/edit, and rollback.

In Coverage tab:
- User can type intent such as "prioritize week 1-3 over labs".
- System returns parsed intent, proposed changes, and expected impact.
- User confirms before risky changes are applied.

## Core Workflow
1. Ingestion finishes and persists notes/sections/components.
2. Coverage analyzer inspects fresh metadata (paths, tags, labels, concepts).
3. Suggestion engine produces weighted group proposals with evidence.
4. Policy engine decides auto-apply vs approval-required.
5. Applied changes are versioned with provenance and rollback snapshot.
6. Quiz generation consumes latest approved/applied scope.

## Policy and Safety
Default behavior:
- Auto-apply only high-confidence suggestions.
- Require explicit user approval for risky changes.

Risk triggers (examples):
- Proposed change sharply reduces candidate pool.
- Exclude-unmatched is enabled or expanded.
- Low-confidence intent parsing.

Safety requirements:
- Always show what matched and what did not.
- Always provide an explainable diff.
- Always keep rollback history.

## Explainability Requirements
Every suggestion and NL command result should include:
- Why: plain-language rationale
- What: exact scope diff
- Evidence: matched tags/source paths/labels
- Impact: estimated candidate composition shift
- Confidence: score and reasons

## Scope (Initial Release)
Included:
- Post-ingestion coverage analysis
- Suggestion cards with confidence and preview
- NL command input for coverage edits
- Auto-apply policy for high-confidence suggestions
- Audit log and rollback snapshots

Not included (initially):
- Fully autonomous silent updates with no visibility
- Multi-user conflict resolution workflows
- External fine-tuning pipelines

## Success Signals
- Reduced manual coverage edits per class over time
- Higher acceptance rate of suggestions
- Lower rate of invalid/unmatched coverage rules
- Improved quiz distribution alignment with intended focus
- Fewer user complaints about unexpected quiz emphasis

## Rollout Plan (High Level)
Phase 1:
- Add metadata explainers and impact preview surfaces.
- Add post-ingestion suggestion generation.

Phase 2:
- Add policy-based auto-apply and rollback snapshots.
- Add suggestion review and audit UI.

Phase 3:
- Add NL coverage refinement endpoint and conversational UI.
- Tune confidence thresholds with telemetry.

## Open Product Questions
- Confidence threshold defaults per class/profile
- Maximum allowed candidate-pool reduction for auto-apply
- UX behavior when suggestions conflict with manual rules
- Whether focused profile should use same auto-tuning policy
