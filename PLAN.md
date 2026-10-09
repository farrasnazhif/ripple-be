# Ripple implementation plan

Source: [PRD.md](PRD.md), draft dated 9 October 2026. This plan translates the PRD into implementation work; unchecked items are not implemented or verified.

## Outcome

Deliver a single-user node canvas that imports a brief and storyboard, interprets a director's note, identifies affected shots, proposes partial revisions, and records the producer's decisions without losing earlier work. Demonstrate Time through impact and preservation, Cost through the regeneration plan and selected Higgsfield jobs, and People through version history and handoff.

## Current foundation

The repository already provides Go/Chi HTTP routing, PostgreSQL access, request validation, JSON responses, email/password authentication, and profiles. Reuse its handler → service → repository structure and existing helpers.
The product frontend uses Next.js in the sibling `../ripple-fe` project. This repository holds the Go backend. Keep each Go feature in its own file per needed layer (`graph_handler.go`, `graph_service.go`, `graph_repository.go`, etc.); use small shared primitives only where necessary. Provider credentials stay on the Go server.

The backend now has owner-scoped project create/list/detail endpoints, a synthetic graph endpoint, deterministic impact preview, brief drafts, R2 attachment storage, a ChatGPT summary draft endpoint, and persisted storyboard shots plus owner-scoped Higgsfield generation jobs (migration 24). The project canvas supports summary review, manual storyboard entry, explicit generation, resumable polling, and output modals. Shot links, director-note decisions, partial revisions, shot versions, and token ledger remain to build. Existing `sessions`, `prds`, and `messages` tables describe a separate document workspace. Login and immediate registration are implemented; billing is outside this scope.

## Decisions before AI work

- [x] Record Go/Chi, PostgreSQL, Next.js, ChatGPT/OpenAI API for text, Higgsfield for media, and Cloudflare R2 for brief attachments as Farras's stack choice (D4).
- [ ] Choose the ChatGPT model and confirm API access and quota (D4, R-09).
- [ ] Choose the Higgsfield operation/model, confirm API access and balance, and complete one live job (D4, R-10).
- [ ] Choose demo hosting; keep API keys server-side and configure them through environment variables.
- [ ] Confirm proposed acceptance thresholds: fixture recall 100%, unaffected text preservation 100%, precision reported without a minimum threshold (D5).
- [ ] Resolve confidence handling from fixture results (D8); require human confirmation regardless of confidence.

Provider access blocks live AI/media work, but does not block schema, deterministic logic, manual input, or labeled seeded demonstrations. Retry thresholds and PDF support do not block the core build.

## Implementation order

### 1. Domain and synthetic fixture

Requirements: FR-001, FR-004, FR-010 foundations.

- [ ] Add migrations for `brief`, `brief_point`, `shot`, `shot_version`, `change`, `decision`, and `token_log`, following PRD section 14.
- [x] Store brief attachment bytes in private Cloudflare R2; keep object key, file name, type, and size in PostgreSQL (migration 19).
- [x] Add Go endpoints for signed PUT, verification/completion, and short-lived signed GET. Check brief ownership on every endpoint.
- [ ] Treat text-only, attachment-only, and mixed brief input as valid; do not claim file contents were interpreted until type-specific extraction exists.
- [ ] Add `generation_job` for Higgsfield requests, status, media URL, and error, linked to decision and shot version.
- [x] Define an owned project boundary with create, list, and detail records. Each project has a linked brief for R2 attachments; project canvas nodes now use each project's own confirmed summary, saved shots and completed outputs.
- [ ] Store immutable raw brief text and append-only shot versions. Store text parts and change diffs as JSONB, with stable point keys.
- [ ] Enforce unique shot codes within a project, version numbers within a shot, and one decision per shot/change. Retain foreign-key relationships for provenance.
- [x] Add entities, request/response DTOs, repositories, services, handlers, and route wiring for impact preview and brief/attachment flows.
- [ ] Seed the synthetic Laju brief, four points, twelve shots, confirmed links, and S1/S2/S3 notes from PRD section 21.
- [ ] Keep the answer key separate from runtime fixture input and freeze it before evaluation.

Done when: the fixture can be read through the API, each shot has an initial version, and stored text matches the source exactly.

### 2. Deterministic impact calculation

Requirement: FR-007. Build this before connecting an LLM.

- [ ] Accept manually entered diffs and validate that each old value matches the current point value.
- [ ] Apply only confirmed diffs and confirmed links; block unresolved point contradictions.
- [ ] Mark directly linked shots as `Needs update`; a changed global point affects all applicable shots, respecting exclusions.
- [ ] Preserve pending/ambiguous diffs without applying them. During a partially confirmed change, mark shots linked only to pending points as `Ambiguous`; otherwise use `Unchanged`.
- [ ] Return every affected shot with point keys, source quotes, and change reasons, including the global-only explanation.
- [ ] Allow a producer to flag a missed shot manually with a recorded reason.
- [ ] Handle zero affected shots and multiple reasons for one shot without duplicate decisions.

Done when: S1 flags 2, 9, and 12; S2 flags all twelve; S3 requests clarification and applies no changes. Verify global-only shots, unconfirmed links, exclusions, and mixed confirmed/pending diffs.

### 3. Director-note interpretation and AI ledger

Requirements: FR-006, FR-011; AI-3.

- [ ] Extend the server-side ChatGPT/OpenAI API integration to structured text operations and a persisted token ledger. A brief-summary client and draft endpoint exist; they return usage but do not persist it yet.
- [ ] Interpret notes using current point keys, values, and scopes rather than resending the whole brief.
- [ ] Return a draft diff or an ambiguity explanation for human review; never apply the draft automatically.
- [ ] Validate keys and old values against stored data when confirming, then persist updated point values and the confirmed change atomically.
- [ ] Retry failed/invalid AI output once, then expose manual diff entry. Record each attempt, including failures; distinguish unavailable usage from measured zero tokens.
- [ ] Expose per-change token totals and call counts. Associate setup calls with their project/brief even when no change exists yet.

Done when: a real S1 note produces a reviewable diff, S3 remains ambiguous, and unavailable AI still permits the manual flow with failure records.

### 4. Partial proposals, decisions, and versioning

Requirements: FR-008, FR-009, FR-010; AI-4 and AI-5.

- [ ] Replace explicit old values deterministically inside affected parts; batch the remaining affected parts into one rewrite call per change.
- [ ] For global-only impact without an explicit linked part, show the reason and request manual revision rather than silently changing unrelated text.
- [ ] Reassemble proposals from parts and assert that every unaffected part remains byte-identical. Reject empty or invalid output and offer manual editing.
- [ ] Provide revision notes for traditional shots and revised prompts for AI shots.
- [ ] Implement `refine` with a reviewed prompt and `keep` with a reason.
- [ ] Persist decision, actor, timestamp, reason, and new version reference. Refine append a version; keep records a decision without creating one.
- [ ] Save decision and version changes in one database transaction; reject duplicate decisions and stale version submissions.
- [ ] Reject a new change while affected shots still have pending decisions, as required by the PRD's single-user conflict rule.

Done when: S1 updates shots 2 and 9, keeps shot 12, retains all old versions, and preserves unaffected text. A repeated submission creates no extra decision or version.

### 5. Brief and storyboard setup

Requirements: FR-001 through FR-005; AI-1 and AI-2. Replace seeded setup after the core change loop works.

- [ ] Accept nonempty brief text and store it exactly as submitted in a new brief version.
- [ ] Extract points with stable keys, categories, scopes, and source quotes; verify each quote is an exact substring or label it unverified.
- [ ] Support editing, adding, deleting, and confirming points; detect duplicate keys and conflicting values.
- [ ] Import storyboard table/CSV data with shot ID, order, description, method, and optional prompt/image reference. Reject duplicate IDs and missing required fields before committing the import.
- [ ] Batch text splitting/linking and require concatenated parts to reproduce the original description and prompt exactly.
- [ ] Offer manual points and links when AI fails or output is invalid. Clearly display unlinked shots and require link confirmation before use.
- [x] Produce a reviewable ChatGPT brief summary without changing the stored raw brief; save only after explicit confirmation.

Done when: a fresh project can enter the same revision loop without seeded points/links; malformed imports and corrupted text splits are rejected.

### 6. Node canvas and Cost/People views

Requirements: FR-012 and FR-013, priority P0b.

- [ ] Expose a regeneration plan with decision counts by action and method, pending decisions, and avoided generations. After decisions finish, avoided generations = total shots − refine shots.
- [ ] Expose a project decision log linking each note, confirmed diff, affected shot, decision, and resulting version.
- [ ] Expose per-shot handoff data: ordered versions, prior version references, reasons, actor, and timestamps. Retry data is added only if the P1 retry feature is built.
- [x] Create the Next.js app in `../ripple-fe` with a synthetic node canvas, scenario switching, keyboard-selectable graph paths, and an inspector. It uses React Query, Axios, and shadcn/ui.
- [ ] Render persisted brief → point → shot edges and change/decision/job nodes when the backend exposes them; distinguish global applicability from direct links.
- [ ] Select a node to show its source, reason, state, history, and actions. Provide pan/zoom, keyboard navigation, visible focus, and a linear shot list.
- [ ] Coordinate frontend delivery: Change, Update board, and Shot detail panels first; Brief points/Storyboard setup next; Handoff, Decision log, and Generation panels afterward.
- [ ] Group global-only impact separately on the update board and label seeded output and placeholder images.

Done when: S1 shows two reworked shots and ten avoided generations, and another teammate can explain shot 9's current state from its handoff view.

### 7. Higgsfield generation

Requirement: FR-018, after a producer approves an AI shot prompt.

- [x] Implement documented starter image (Soul 2) and video (Seedance 2.0) submit/status calls in a Go server-side client. Model choice for the actual campaign remains open.
- [ ] Submit at most one job per decision; never automatically retry an ambiguous generation POST.
- [ ] Persist provider request ID and poll until terminal status. Keep pending, failed, canceled, and completed jobs visible.
- [ ] Link completed media to the new shot version; retain old versions and media. Retry only on explicit action.
- [ ] Count media jobs and charges separately from ChatGPT tokens.

Done when: an approved shot produces one real result, or the demo labels a sample result and discloses that live generation was unavailable.

### 8. Verification and demo delivery

- [ ] Test S1 end-to-end, S2 global impact, and S3 ambiguity (T-01, T-02, T-06).
- [ ] Report S1 precision 2/3 and recall 2/2; report S2 recall 12/12 against the fixed answer key (T-02, T-03).
- [ ] Check byte preservation and reject edits to old versions and raw briefs (T-04, T-05).
- [ ] Exercise AI failure/manual recovery, duplicate and invalid imports, and invalid part concatenation (T-07 through T-09).
- [ ] Check transactional decision/version storage, stale submissions, duplicate decisions, and pending-decision conflicts.
- [ ] Repeat core regression checks after P0b or P1 changes (T-10).
- [ ] Run usability tasks with the mentor/team and record assistance needed (T-11).
- [ ] Benchmark Ripple against B1 using the same model: whole brief + whole storyboard + director note in one baseline call (T-12).
- [ ] Trace BP3 → shot 9 from the node canvas with keyboard and check the reason against backend data (T-13).
- [ ] Check one Higgsfield job through terminal status and verify repeated clicks cannot create a second job (T-14).
- [ ] Check that ChatGPT summary review never changes raw brief text (T-15).
- [ ] Report tokens, call counts, review time, total workflow time, accuracy, and preservation. Include setup costs amortized across changes; claim savings only if measurements support them.
- [ ] Run two complete demo rehearsals and record a backup video.

Demo order: Time with S1 and a short S2 on the node canvas → Cost regeneration plan and Higgsfield result → People handoff for shot 9 → ChatGPT token ledger and benchmark. Use synthetic assets only; label fallback media clearly.

## Delivery gates and ownership

Use the PRD's milestone times as checkpoints, not as evidence that work is complete.

| Gate | Required result | Owner |
| --- | --- | --- |
| Before mentor test | Seeded data, impact, real note interpretation, partial proposals, decisions, versions, ledger; node canvas and Change/Board/Detail panels | Farras + Rasya |
| After core validation | Real brief extraction and shot linking; manual recovery | Farras |
| Before final rehearsal | Higgsfield generation or labeled fallback, regeneration plan, handoff, decision log, benchmark | Farras + Rasya + Jo |
| Final delivery | Acceptance results, honest demo labels, two rehearsals, backup video | Team |

Jo prepares the independent answer key and business validation; Rasya owns UI and usability; Farras owns backend, provider integration, and technical checks. The Next.js frontend lives in the sibling `../ripple-fe` project.

## Cut line

At the PRD's 03:00 checkpoint, cut retry tracking first, then whole-brief comparison, restore-version actions, and a dedicated benchmark page. Keep benchmark results in a table/deck if needed.

Do not add storyboard generation, automated media quality scoring, automatic method recommendations, billing flows, multi-user collaboration, vector databases, agents, queues, microservices, or OZU integrations.

If live ChatGPT or Higgsfield access is unavailable, use manual entry and seeded output labeled as examples. This supports a fallback demo but does not satisfy acceptance of the live AI or media feature; record that limitation explicitly.

## Completion checklist

- [ ] Core S1 flow succeeds through saved decisions and versions.
- [ ] S2 global impact and S3 clarification behave correctly.
- [ ] Unaffected text and all historical work are preserved.
- [ ] AI outputs require confirmation and failures have manual paths.
- [ ] Cost, People, and token evidence are visible.
- [ ] Node canvas shows confirmed relationships and matches impact calculation.
- [ ] Higgsfield jobs are linked to approved shot versions and remain visible on failure.
- [ ] Setup works without hidden seeded dependencies, or those dependencies are disclosed.
- [ ] Acceptance results and benchmark measurements are recorded.
- [ ] Demo hosting works, keys stay server-side, and rehearsals pass.

## Implemented project flow — 9 October 2026

- [x] Load R2 settings from `.envrc` under Air and check configuration before creating a project with files.
- [x] Capture raw brief text once; review/edit and confirm ChatGPT summaries with manual fallback.
- [x] Use supported image/PDF references in summary inputs; leave other formats as downloadable references.
- [x] Save project-owned storyboard shots manually after summary confirmation, locking that summary version.
- [x] Persist explicit image/video generation requests and provider IDs with ownership checks and duplicate/active-job guards.
- [x] Resume status polling after refresh; retain failed/uncertain history and show media only after completion.
- [x] Verify PostgreSQL ownership, immutable source text, storyboard concurrency boundaries, and duplicate job claims; frontend types/lint/build and node regression checks.
- [ ] CSV storyboard import, revision/version workflow, token ledger, durable R2 output archival, webhook/background polling, provider reconciliation UI, and live provider validation.

The original implementation notes above describe the earlier flow. The updated implementation below supersedes its source/summary locking behavior.

## Implemented summary and impact workflow — 10 October 2026

- [x] Migration 25: brief snapshots, stable requirements, shot links, persisted changes/decisions, accepted media and final output history.
- [x] Editable brief/summary modals, multiple references, removal with archive access, ChatGPT summary refinement, and reviewed manual fallback.
- [x] Choose images or video after summary: generate a final image directly, or review a ChatGPT-drafted/manual video storyboard.
- [x] Calculate local/global/deleted requirement impact; narrative-only edits request manual review.
- [x] Wide media review modal with real preview, version thumbnails, same-prompt regeneration, edited-prompt refinement, acceptance and keep-with-reason.
- [x] Group completed versions into clip/image nodes; connect accepted inputs to final outputs.
- [x] Combine accepted clips with FFmpeg and save final MP4 to R2; collect accepted images.
- [x] Update five demos for the specified owner (including Brief + Brief Summary only and Brief + Brief Summary + Storyboard only) without paid AI calls.
- [x] PostgreSQL workflow checks, impact/assembly assertions, frontend graph checks, type-check, lint and production build.
- [ ] Paid live provider validation. Browser testing stopped at user request.
- [ ] Automatic note interpretation/point extraction, token ledger, retry thresholds, CSV import, background provider reconciliation and durable archival of individual provider outputs.

## Implemented project table — 10 October 2026

- [x] Shadcn table with title, description, creator username, creation date, Edit and Delete.
- [x] Owner-scoped metadata update and project deletion APIs; preserve brief content and project identity when editing.
- [x] Shared shadcn Alert Dialog confirmation for deleting a project, clearing brief/summary text, removing references/requirements, removing planned shots and selected files.
- [x] Migration 26 queues R2 cleanup in the deletion transaction; cleanup retries after signed upload links expire.
- [x] PostgreSQL CRUD/ownership/cascade/cleanup checks, Go tests/vet, frontend type/lint/graph checks and build. Browser testing remains skipped at user request.

- [x] Migration 27: direct final image/video refinement, saved source references and adjustments/reasons, candidate polling, explicit acceptance, version comparison, and duplicate-submission protection.
- [x] Verify owner boundaries, collection preservation, stale brief/source rejection, and accepted-only final canvas projection.

- [x] Direct Brief → Brief Summary → Final Image flow; no image plan or storyboard-shot creation. Preserve existing image collections and continue final refinement/versioning.


### Project media budgeting

Implemented a $50 USD-cent budget default for every existing and new project, configurable image/clip/final-refinement estimates, cost confirmation before media requests, atomic reservations, an 80% allocation warning, and a latest-100 usage ledger. Owners can adjust project caps. Missing rates block those media requests; historical charges are not backfilled. Unknown submissions retain reservations, explicit submission rejections release them, and terminal accepted requests count as estimated spending. Actual provider charges/refunds, ChatGPT costs, agency/monthly caps and shared-team permissions remain outside this first project-budget implementation.
