# Ripple PRD

*October 9, 2026 · @joj · English revision*

This PRD guides the design, implementation, testing, and demo of Ripple for the PENS 2026 Hackathon, Production Survivability track. The source Ripple document retains the underlying research and process. Requirements are proposals unless identified as a team decision.

## 1. Document control

| Item | Detail |
| --- | --- |
| Product | Ripple |
| Version | 1.2, incorporating Farras's architecture, provider, storage, and interface decisions |
| Status | Draft; team approval pending |
| Date | October 9, 2026 |
| Owner | [DATA NOT AVAILABLE] Proposed: Jo, research and business |
| Readers | Rasya, UI/UX; Farras, engineering; Jo, research and business |
| Tier 1 sources | PENS 2026 hackathon terms; OZU slides 13–30 from the October 9 briefing; event schedule |
| Tier 2 sources | OZU mentor interview at 16:15; Ripple source document; confirmed team decisions |
| Open decisions | D3, model/credential/hosting details of D4, D6, D8, D9 (section 22) |

Evidence labels: [VERIFIED FACT], [USER-PROVIDED CLAIM], [INFERENCE], [ASSUMPTION], [PROPOSED REQUIREMENT], and [DATA NOT AVAILABLE]. The wider PENS guidebook, judging weights, submission format, and pitch duration/language were not available in the sources.

## Confirmed product flow — 10 October 2026

The primary features are **Brief Summary** and **Change Impact Tracking**. The following user decisions supersede older research proposals below wherever they conflict.

- `/project` displays an owned-project table with title, description, creator, creation date, and Edit/Delete actions. Critical destructive actions require an alert dialog confirmation. `/project/[projectId]` displays the node canvas beside the shared sidebar and beneath the project-name header. Initially only Brief exists; later nodes require saved outputs.
- Brief and Brief Summary open editable modals: edit/delete content, add/remove references, review requirements, save a new immutable snapshot, or refine the summary with ChatGPT. Earlier source text and references remain accessible in history.
- After confirming the summary, choose **Images** or **Video**. Images use a reviewed image plan; video uses a reviewed storyboard. Plans can be manually entered or drafted with ChatGPT. Requirements have stable keys and global/local scope; local requirements are explicitly linked to shots.
- Brief edits produce stored requirement diffs and deterministic affected/unchanged shot lists with reasons. Global requirements affect every shot; local requirements affect linked shots. Narrative-only edits conservatively request review instead of claiming no impact. Requirement deletion retains historical shot links for impact calculation.
- Each shot/output type has one canvas node showing real media, brief version, approval/review state, and version count. The wide clip/image modal shows saved summary, current linked requirements, impact reasons, Generate Again, Adjust/Refine, Mark as Accepted, and selectable version thumbnails. Earlier media is preserved.
- **Refine** submits an explicitly approved edited prompt as another version. **Keep** preserves the selected version with a required reason. Accepted affected outputs cannot enter a new final until their impact is resolved.
- Final outputs can be refined directly from their modal using the selected saved image/video as the Higgsfield reference. Producers provide adjustments and a reason, compare a candidate against its source, and explicitly accept it before the canvas changes. Image collection edits preserve other members. Brief/source approval checks and idempotency apply; previous finals and source nodes remain in history.
- Accepted images form a final collection. Accepted clips are normalized and combined in storyboard order with FFmpeg; the resulting MP4 is stored privately in R2. Final output snapshots retain selected job IDs and brief version. Earlier finals remain accessible.
- The five synthetic demos show a raw brief, a reviewed/refined image collection, a video project whose local lighting revision affects Clip 1 while preserving Clip 2, a two-node Brief → Brief Summary project, and a three-node Brief → Brief Summary → Storyboard project without generated media. Samples are labeled and never call paid generation providers during seeding.

Current limits: automatic requirement extraction/director-note interpretation, CSV import, token ledger, retry thresholds, and exact text-part proposals remain separate work. Requirement links and refinements are producer-confirmed. Individual generation URLs remain provider-hosted; assembled final videos and source attachments use R2; directly refined final media currently remains provider-hosted.

## 2. Executive summary

Ripple preserves existing creative work when a brief or direction changes. It turns a brief into traceable brief points, links storyboard shots to those points, and shows which shots a director's note affects, why, and which parts may need revision. The producer chooses refine or keep; Ripple stores the decision and its reason.

The main page at `/project` lists the signed-in user's projects and creates new projects in a modal with title, description, and multiple brief files; `/project/[projectId]` holds that project's **node canvas**. The shared header shows the page or project name, the account dropdown sits in the sidebar, and node details open in a modal. Briefs, brief points, shots, changes, decisions, and generation results appear as connected nodes. Next.js renders and edits the graph; the Go backend stores authoritative relationships and calculates impact. ChatGPT through the OpenAI API handles text understanding and drafting, including brief summaries. Higgsfield generates producer-approved images and videos. Cloudflare R2 stores original brief attachments; PostgreSQL stores raw text and attachment metadata. These architecture and interface choices come from Farras [USER-PROVIDED CLAIM]. The first frontend implementation lives in the sibling `../ripple-fe` project. Project records and linked briefs are now persisted and owner-scoped. The initial canvas shows only Raw Brief; downstream nodes appear only when the corresponding project output exists. Brief Summary currently appears after a nonempty summary is saved; Saved image plans/storyboards, media versions, acceptance, change impact tracking, and final outputs are implemented.

OZU reports that shots made for an old direction become stale, late changes cause rework, retries consume budget, and knowledge sits with a few people [VERIFIED FACT: slides 13, 14, 21, 27]. OZU already has a brief-to-storyboard app [VERIFIED FACT: mentor interview], so Ripple focuses on the moment of change.

The MVP uses synthetic data to demonstrate affected-shot mapping and partial updates (Time), a regeneration plan and selected Higgsfield jobs (Cost), and a decision log and handoff history (People). ChatGPT token use is measured against a baseline. Still unvalidated: how often changes are partial, whether OZU's app loses director edits, and who would adopt or fund Ripple.

## 3. Problem definition

The inferred root problem is the absence of a traceable relationship between each part of a brief and each shot. Without it, a team cannot reliably identify the impact of a change.

| Element | Detail | Evidence |
| --- | --- | --- |
| Primary user | Producer in a small AI VFX or creative team | [VERIFIED FACT: slides 23, 26] |
| Main problem | The team cannot tell which shots and shot parts a direction change affects | [INFERENCE: slides 13, 14, 27] |
| Current workflow | OZU's app turns a brief into a storyboard and prompts; the director edits; shots are made with AI or traditional methods and combined | [VERIFIED FACT: mentor interview] |
| Trigger | Director feedback or a new brief after shot work has started | [VERIFIED FACT: slide 14] |
| Workaround | Repeat shots, guess the impact, or ask the person who knows | [INFERENCE: slides 15, 21] |
| Consequences | Rework, repeated generation cost, late edits, and dependence on a few people | [VERIFIED FACT: slides 14, 21] |
| Missing evidence | Wasted-shot fraction, director-edit preservation, and retry frequency | [DATA NOT AVAILABLE] |

## 4. Goals and non-goals

G1–G7 are MVP goals. G5 requires measurement; it does not promise token savings.

| ID | Goal | Measure |
| --- | --- | --- |
| G1 | Identify affected shots correctly | Recall and precision against an independent answer key |
| G2 | Explain every flagged shot | Share with point key, source quote, and change note visible |
| G3 | Preserve unaffected content | Byte-for-byte comparison |
| G4 | Let producers use the flow without training | Tasks T1–T3 completed unaided |
| G5 | Measure token use against baseline B1 | Tokens per change and amortized over repeated changes |
| G6 | Show avoided generations | Total shots minus refine shots |
| G7 | Enable handoff without the producer | Task T5 completed unaided |
| G8 | Reduce real OZU rework time | Future pilot on a real project |

MVP non-goals: automatic image/video quality scoring; automatic choice between AI and traditional production; multi-user collaboration or real-time editing; OZU internal integration; and use of client footage. The frontend supports email/password registration and login. Registration activates the account immediately and returns a JWT; workspace and brief screens require a valid JWT before rendering. There is no OTP, email delivery, Google OAuth, or PayPal billing flow.

## 5. Users and stakeholders

| Stakeholder | Role and need | Adoption concern |
| --- | --- | --- |
| Producer | Primary user; identify affected shots, decide revisions, and meet deadlines | Setup time and trust in automatic linking |
| Director | Source of direction changes; needs interpretation to match intent | System must not replace creative decisions |
| Artist/editor | Secondary user; needs specific shot revision instructions | Another tool in the workflow |
| Intern/new team member | Needs version history and reasons to continue work | Missing or incomplete decisions |
| OZU | Sponsor and possible user; needs measurable time, cost, and knowledge benefits | Existing internal app could add similar features |
| AI providers | ChatGPT for text and Higgsfield for media | Model access, limits, credentials, and cost |

## 6. User stories

P0 is the core flow, P0b follows the first mentor test, and P1 is optional.

| ID | Story | Priority |
| --- | --- | --- |
| US-01 | Paste or attach a brief and obtain traceable brief points | P0 |
| US-02 | Correct brief points and mark each global or local | P0 |
| US-03 | Import an existing storyboard | P0 |
| US-04 | Correct links from shot parts to brief points | P0 |
| US-05 | Paste a director's note to propose a change | P0 |
| US-06 | Confirm or clarify the note interpretation | P0 |
| US-07 | See the status of every shot | P0 |
| US-08 | Inspect the source and reason for a flagged shot | P0 |
| US-09 | Compare shot content before and after a proposal | P0 |
| US-10 | Choose refine or keep per affected shot | P0 |
| US-11 | Read all previous shot versions | P0 |
| US-12 | See the regeneration plan and expected rework | P0b |
| US-13 | Read a decision log and handoff view | P0b |
| US-14 | Record retries against the brief point that failed | P1 |
| US-15 | Compare a new brief version with the current one | P1 |
| US-16 | Restore an earlier shot version as a new version | P1 |
| US-17 | Trace brief point → shot → change connections on a node canvas | P0 |
| US-18 | Generate an image or video only for a selected, approved AI shot and see its result on that shot | P0b |

## 7. Scope and priority

| ID | Feature | Priority | Dependency and delivery rule |
| --- | --- | --- | --- |
| F-01 | Brief input and point extraction | P0 | Seeded before first mentor test; ChatGPT extraction afterward |
| F-02 | Storyboard import and links | P0 | Seeded before first mentor test; ChatGPT linking afterward |
| F-03 | Director-note interpretation | P0 | Reviewable ChatGPT draft |
| F-04 | Impact propagation and shot status | P0 | Deterministic Go logic |
| F-05 | Partial update proposal | P0 | Deterministic replacement plus targeted ChatGPT drafting |
| F-06 | Refine/keep decisions | P0 | Producer controls the choice |
| F-07 | Shot versions with actor and reason | P0 | Append-only history |
| F-08 | ChatGPT token ledger | P0 | Every text-model attempt |
| F-09 | Regeneration plan | P0b | After decisions |
| F-10 | Decision log and handoff | P0b | Based on versions and decisions |
| F-11 | Retry log and discussion threshold | P1 | First feature cut at the 03:00 checkpoint |
| F-12 | Diff against a new brief version | P1 | Reuses point extraction |
| F-13 | Restore version | P1 | Append a new version |
| F-14 | Dedicated benchmark page | P1 | Results can appear in the pitch deck |
| F-15 | Higgsfield image and video generation | P0b | Real job if credentials/access work; otherwise explicitly labeled example |
| F-16 | Node canvas | P0 | Shows confirmed data relationships and impact |

P2: OZU integration, budget/deadline retry limits, and multi-user collaboration. Do not add a vector database, agents, queue, or microservices for this MVP.

## 8. Core user journey

| Step | Producer action | System response | Data and failure handling |
| --- | --- | --- | --- |
| J1 | Paste text and/or attach brief files | Store exact text in PostgreSQL, original files in private R2; extract summary and points where supported | Brief, attachment, brief point; reject empty finalized brief and invalid AI output |
| J2 | Correct point values and scopes | Save confirmed points; show contradictions | Conflicting values block J5 |
| J3 | Import storyboard | Validate rows and link text parts | Shot and v1; reject duplicate IDs and missing required fields |
| J4 | Correct shot links | Display unlinked shots | Unconfirmed links never drive propagation |
| J5 | Paste director note | Propose a point diff or mark ambiguity | Failed ChatGPT call retries once, then offers manual diff |
| J6 | Confirm diff | Calculate impact, status, and reasons | Reject stale old values |
| J7 | Review affected shot and choose action | Store decision; append a version for refine | Invalid proposals require manual editing; a decision cannot be applied twice |
| J8 | Open log or handoff view | Display complete history | Read-only |
| J9 | Approve image/video prompt for an AI shot | Submit Higgsfield job and show status/result on the shot | Failure keeps previous work; retry requires an explicit action |

Ambiguous notes prompt clarification rather than propagation. AI failure offers a labeled manual path. A seeded demo must disclose which points and links were prepared by the team.

## 9. Functional requirements

All ChatGPT output is reviewed before use. A Higgsfield job is submitted only after prompt approval. The acceptance criteria below are proposed unless the team confirms them.

| ID | Requirement | Priority | Behavior and acceptance |
| --- | --- | --- | --- |
| FR-001 | Brief input | P0 | Accept text, attachments, or both. Store raw text byte-for-byte in PostgreSQL and original attachment bytes in R2. Reject a finalized brief with neither. Every version has an ID. |
| FR-002 | Brief point extraction | P0 | One ChatGPT call yields key, category (color_palette/story/timeline/other), value, global/local scope, and source quote. Validate JSON; retry once, then allow manual entry. Mark quotes not found exactly in the source. |
| FR-003 | Point review | P0 | Add, edit, or remove points. Duplicate keys with conflicting values must be resolved before changes are processed. |
| FR-004 | Storyboard import | P0 | Accept table/CSV with shot_id, order, description, AI/traditional method, optional prompt and image reference. Reject duplicate IDs or missing required columns; save valid rows as v1. |
| FR-005 | Shot links | P0 | Batch-split description and AI prompt into parts, each linked to one point key or null. Concatenated parts must exactly equal the original text; invalid output goes to manual linking. Global points apply to all shots unless explicitly excluded. |
| FR-006 | Note interpretation | P0 | ChatGPT receives the note and point list, not the full brief, and returns key/old/new/confidence diffs or an ambiguity reason. Do not apply unconfirmed diffs. Retry once on failure, then allow manual diff. Reject mismatched old values. |
| FR-007 | Propagation | P0 | Deterministically flag shots with confirmed direct links to changed keys or applicable global keys. Pending applicable diffs produce `Ambiguous` unless another confirmed diff already makes the shot `Needs update`. Every flagged shot includes a point and reason, including a global-only reason. |
| FR-008 | Partial proposals | P0 | Replace explicit old values in affected parts; send remaining parts in one batch ChatGPT call per change. Produce revision notes for traditional shots and revised prompts for AI shots. Reject empty or unrelated changes; unaffected parts remain byte-identical. |
| FR-009 | Producer decisions | P0 | Refine with a reviewed prompt or keep with a recorded reason. Store actor and time; reject a second decision for the same shot/change. |
| FR-010 | Versions | P0 | Refine append a shot version linked to parent and change; keep records a decision without a new version. Earlier versions cannot be edited. |
| FR-011 | Token ledger | P0 | Record purpose, model, input/output tokens, latency, change, success/failure for every ChatGPT call, including failures. Display accurate totals and call counts. |
| FR-012 | Regeneration plan | P0b | Count shots by action/method; avoided generations = total shots − refine shots after decisions are complete. |
| FR-013 | Decision log and handoff | P0b | Show change, diff, affected shots, decisions, actors, and per-shot versions without an AI call. |
| FR-014 | Retry log | P1 | Record shot, version, failed point, note, actor. Mark a discussion threshold after N repeated point failures, M shot retries, or the same failure across K shots. |
| FR-015 | New brief diff | P1 | Extract v2, match stable keys deterministically, and ask ChatGPT only about unmatched points. Feed confirmed diff into FR-006. |
| FR-016 | Restore | P1 | Append a reasoned restore version; retain all previous versions. |
| FR-017 | Node canvas | P0 | Next.js displays brief, point, shot, change, and decision nodes with stored connections. Select a node for source/status/actions; support pan/zoom, keyboard selection, visible focus, and a linear shot list. S1 must show BP3 → shots 2/9/12. |
| FR-018 | Higgsfield generation | P0b | After producer approval, Go submits one image or video job for the selected AI shot. Store the provider ID and state; poll to a terminal result. One approved decision creates at most one job. Failed or slow jobs stay visible and never erase old versions/results. |
| FR-019 | ChatGPT brief summary | P0 | Provide a concise draft for review. Keep raw text unchanged and save a summary only after explicit confirmation. Attachment contents are summarized only after type-specific extraction is implemented. |
| FR-020 | R2 attachments | P0 | Go signs a private PUT for a new object key; after upload it checks the object and saves metadata. Signed GET requires an ownership check. Browser never receives R2 credentials. Failed or expired uploads remain unsaved. Current backend limit: 25 MiB per file. |

Values for N, M, and K remain open under D3 (section 22).

## 10. Subsystem rules

Only confirmed direct links or explicit global scope cause propagation. Similarity alone never creates an impact edge.

| Subsystem | Rule |
| --- | --- |
| Brief input | Text is P0; original file upload is P0; PDF text extraction remains P1 (D6). Never mutate source text or bytes. A changed brief creates a new version. |
| Brief attachments | Keep private originals in Cloudflare R2. PostgreSQL holds key, name, type, size, and brief relationship. An unsupported file remains accessible as an attachment but is not silently interpreted by ChatGPT. |
| Brief points | Each point has ID, stable key, category, value, scope, source quote, and brief version. The producer resolves duplicates and contradictions. Preserve ambiguous source requirements with an ambiguity label. |
| Shots | A shot holds code, order, method, and image reference; each version holds description, prompt, and linked parts. Concatenated parts must equal source text. Unconfirmed links are ignored by propagation. |
| Interpretation | Show confidence and require human review; set a low-confidence threshold after observing fixture output (D8). |
| Propagation | A changed global point flags an applicable shot without a direct text link, with the message “Applies to every shot; not mentioned directly in the description.” |
| Partial proposals | ChatGPT sees only an affected part, diff, and one neighboring part when string replacement is insufficient. |
| Decisions and versions | Refine/keep are producer choices. Undo is an append-only restore version (P1). Versions retain change, decision, actor, and time. |
| Media generation | ChatGPT drafts text; Higgsfield generates only approved image/video jobs. Keep provider state and media separate from ChatGPT token usage. |

## 11. Business rules and edge cases

| Case | Required behavior |
| --- | --- |
| Contradictory brief requirements | Flag and resolve before processing a change. |
| Ambiguous director note | Do not propagate; ask for clarification. |
| Shot with no confirmed link | Flag only through applicable global points. |
| One point affects many shots | Flag each with the same source reason. |
| One shot is affected by multiple points | Return one status, all reasons, and one producer decision. |
| Global change | Flag all applicable shots; revise only affected parts. |
| No shots affected | Show an empty-result message and retain the diff. |
| False positive | Producer can keep the shot and record why. |
| False negative | Producer can flag a missed shot manually; disclose the limitation in the pitch. |
| Invalid proposal or rejected AI output | Reject or allow manual edit/keep. |
| AI failure | Retry a text call once, then allow manual entry and log the attempt. Do not blindly retry a paid Higgsfield generation POST after an ambiguous timeout. |
| Version conflict | Single-user MVP rejects a new change while affected shots await decisions. |
| Duplicate Generate click | One job per decision or idempotency key. |
| Failed or slow Higgsfield job | Show status; preserve previous versions and media; retry only on explicit action. |
| Restore earlier version | Append a new restore version (P1). |
| R2 upload mismatch or interruption | Keep attachment pending, reject completion, and remove a mismatched object. |

## 12. Non-functional requirements

| ID | Requirement | Acceptance method |
| --- | --- | --- |
| NFR-01 | Integrity: original brief text/files and historical shot versions are immutable | T-05 and T-16 |
| NFR-02 | Auditability: every impact and version traces to a source and decision | All flagged fixture shots have complete reasons |
| NFR-03 | Recovery: every ChatGPT operation has a manual path | T-07 |
| NFR-04 | Response time | [DATA NOT AVAILABLE] Measure median fixture change time before setting a target |
| NFR-05 | Privacy | Use synthetic/public assets only; no OZU client footage |
| NFR-06 | Secrets | ChatGPT, Higgsfield, and R2 credentials stay on the Go server and outside the repository |
| NFR-07 | Observability | Token ledger and error logs for text calls; job state for media calls |
| NFR-08 | Token efficiency | Measure and report against B1, without assuming improvement |
| NFR-09 | Accessible graph | Keyboard access, visible focus, text status labels, and a linear shot list |

## 13. AI and token-efficiency strategy

ChatGPT handles language tasks. Go handles validation, propagation, status, counts, and direct string replacement. Higgsfield handles producer-approved media generation.

| ID | ChatGPT operation | Input and output | Frequency and validation |
| --- | --- | --- | --- |
| AI-1 | Extract brief points | Raw text → structured JSON points | Once per brief version; validate schema and exact source quotes |
| AI-2 | Split and link shot text | All shot descriptions/prompts + point list → parts | Once per import, batched; concatenation must exactly match source |
| AI-3 | Interpret director note | Note + point list → diff or ambiguity | Once per change; old values must match stored points |
| AI-4 | Rewrite affected parts | Part text + diff + one neighbor → replacement text | One batch per change for parts not handled by direct replacement |
| AI-5 | Refine a generation prompt | Existing prompt + confirmed changes → revised prompt | Producer approves each refinement; reject empty output |
| AI-6 | Summarize brief | Text brief → short review summary | Once per brief version; producer confirms before saving |

Structured text output is validated. A failed text call is retried once and then falls back to manual entry. File contents are never claimed as interpreted until a type-specific extraction path exists.

Baseline B1 is one ChatGPT call containing the entire brief, entire storyboard, and director note, asking for affected shots and new text. Use the same model as Ripple. Measure tokens, call counts, human review time, total flow time, recall, precision, unchanged-part preservation, and human quality review. Report cost per change and amortized over *k* changes, including AI-1/AI-2 setup, plus the break-even point. Report a higher one-change cost honestly. Token savings cannot come at the expense of recall. Higgsfield job counts, duration, and available charges are reported separately from ChatGPT tokens.

## 14. Data model and architecture

The backend uses Go, Chi, and PostgreSQL. The node canvas uses Next.js. The frontend consumes Go APIs and holds no provider secrets. In Go, each feature has one focused file per required layer (for example, `brief_handler.go`, `brief_service.go`, `brief_repository.go`); dependencies flow handler → service → repository through interfaces. Shared entities/DTOs describe contracts. Separate feature logic is not imported merely to avoid a small local function. This is a file organization rule, not a request for one giant file per feature.

Target relational model:

```text
brief(id, project_id, version, raw_text, summary, created_at)
brief_attachment(id, brief_id, object_key, file_name, content_type, size_bytes, status, created_at)
brief_point(id, brief_id, key, category, value, scope, source_quote, quote_verified, status)
shot(id, project_id, shot_code, order, method, image_ref)
shot_version(id, shot_id, n, description, prompt, parts_json, parent_id, change_id, reason, decided_by, created_at)
  parts_json: [{part_id, text, point_key_or_null, confirmed}]
change(id, project_id, source_type, note_text, diffs_json, status, ambiguous_reason, created_at)
  diffs_json: [{point_key, old, new, confidence, confirmed}]
decision(id, change_id, shot_id, action, proposal_text, final_text, new_version_id, reason, decided_by, created_at)
token_log(id, purpose, model, tokens_in, tokens_out, latency_ms, change_id, ok, created_at)
generation_job(id, shot_version_id, decision_id, provider_request_id, model, status, media_url, error, created_at, updated_at)
retry(id, shot_id, shot_version_id, point_key, note, decided_by, created_at)     P1
project_settings(project_id, retry_n, retry_m, retry_k)                         P1
```

The current backend has owner-scoped `projects`, a `briefs` draft table tied to existing users, and `brief_attachments`; migration 25 adds brief snapshots, requirements, shot links, changes, decisions, accepted generation versions and final output snapshots. Briefs are linked to their owned projects. Existing `sessions`, `prds`, and `messages` are a separate old document workspace and must not be repurposed silently. Earlier brief snapshots and completed attachment bytes are immutable; edits append a new brief version. Older shot versions are append-only. Stable point keys link across versions; point-to-shot links belong to the versioned parts. A graph node/edge is a projection of stored domain data, not an independent source of truth. The synthetic graph endpoint is demo data only.

Higgsfield's REST API accepts separate image and video model paths and asynchronous request IDs. The Go client can submit image/video prompts and poll status; the production job API requires a persisted, owner-checked decision and shot version. The starter models are Soul 2 for images and Seedance 2.0 for video; final model choice, account access, and hosting remain open (D4). The private R2 bucket uses short-lived signed PUT/GET URLs after ownership checks. No vector database, agent orchestration, or microservices are needed. Polling suffices for MVP job status; add a queue only if actual load warrants it.

## 15. UX and interface requirements

The node canvas is inside the project detail page. S1–S8 below are focus panels or modes on that canvas, not eight unrelated pages. Brief → point → shot shows confirmed direct relationships; a change node highlights affected paths; decision and generation nodes show actions and results. Global applicability has a separate indicator so it cannot be mistaken for a confirmed direct edge. Node position is a display preference, never the source of relationship truth. The current frontend implements a project list, project detail with synthetic graph, and brief/auth flows; S1–S8 remain the target for persisted project data.

| Panel | Purpose | Main content and action | Empty or failure state |
| --- | --- | --- | --- |
| S1 Brief points | Review brief understanding | Points, source quotes, scope; confirm/edit | Prompt to add a brief; manual input after AI error |
| S2 Storyboard | Review shots and links | Shot, method, linked parts; confirm links | Prompt to import; explain rejected rows |
| S3 Change | Interpret note | Old/new diff and confidence; confirm | Ask for clarification on ambiguity |
| S4 Update board | See impact | Group shots by direct/global-only reason, status, regeneration plan, token line | “No shots affected” when applicable |
| S5 Shot detail | Choose action | Source, reason, before/after, refine/keep | Manual edit if proposal is invalid |
| S6 Handoff | Read one shot's history | Versions, reason, actor, retry | “No changes yet” |
| S7 Decision log | Read project history | Change, diff, shot, decision; open shot | “No changes yet” |
| S8 Generation | Run approved media job | Prompt, model, status, image/video result; explicit retry | “Not generated”; show job failure without losing earlier work |

The canvas supports pan/zoom. Every node and edge can be selected by keyboard, focus remains visible, and details never rely on color alone. A linear shot list offers alternate navigation. Manually adding an edge changes a stored link only after validation and confirmation. Use the UI terms “Brief points,” “Needs update,” “Unchanged,” and “Ambiguous.” Refine creates another media version; keep preserves the selected version with a reason.

## 16. Acceptance criteria and test plan

All fixtures are synthetic and independent of OZU client data. Jo defines the answer key before running the system. TP = correctly flagged required change, FP = flagged but unnecessary, FN = required but missed. Precision = TP/(TP+FP); recall = TP/(TP+FN). The pass thresholds remain proposed under D5.

| ID | Test | Expected result |
| --- | --- | --- |
| T-01 | S1 end-to-end | Completes without error |
| T-02 | S1 and S2 accuracy | 100% recall on the small fixture |
| T-03 | S1 and S2 precision | Report actual precision; FP can be handled with keep |
| T-04 | Unaffected shot parts | 100% byte-identical |
| T-05 | Attempt to edit prior version or raw brief | Rejected |
| T-06 | S3 ambiguous note | No propagation; clarification requested |
| T-07 | Disable ChatGPT key | Manual path appears and ledger records failure |
| T-08 | Duplicate shot ID or missing required import field | Row rejected with clear reason |
| T-09 | Corrupt AI-generated text-part concatenation | Rejected |
| T-10 | Rerun T-01 through T-04 after P0b/P1 work | Still passes |
| T-11 | Mentor tasks T1–T3; internal tasks T4–T7 | Completed without help |
| T-12 | Ripple versus B1 token benchmark on S1/S2 | Per-change, amortized, and break-even results reported |
| T-13 | Traverse BP3 → shot 9 on graph by keyboard | Path and reason match backend data |
| T-14 | Higgsfield image/video job; duplicate submit | One job per decision; terminal result/error visible; older work intact |
| T-15 | Generate and confirm summary | Original text remains byte-identical |
| T-16 | R2 upload, complete, list, download, wrong owner, wrong size | Only owner can access; mismatch rejected; bytes stored outside PostgreSQL |

## 17. Success metrics

A smooth demo alone does not establish accuracy or user value. Measure each outcome against its own baseline.

| Category | Metric | Procedure | Baseline |
| --- | --- | --- | --- |
| Correctness | Recall and precision | T-02, T-03 | Prewritten answer key |
| Correctness | Traceability | Share of flagged shots with a complete reason | Target 100% |
| Correctness | Preservation | T-04 | Previous version text |
| User | Time to process one change | Time from receiving the note to all decisions | Manual review of every shot with the same documents |
| User | Review load | Number of shots opened | Every shot |
| User | Understanding and control | Tasks T2 and T3 | Observed manual workflow |
| Operations | AI calls, tokens, processing time | Token ledger | B1 baseline |
| Operations | Avoided generation | Regeneration plan | Regenerating every shot |
| Competition | Complete core demo and evidenced claims | Two rehearsals; T-02, T-04, T-12 | Section 23 traceability matrix |

## 18. Business value and validation

Ripple's value to OZU cannot yet be quantified. Its potential benefits are fewer unnecessary shot reruns, fewer stale shots entering review, and durable reasons for decisions across handoffs. Its added costs are brief-point and link setup, AI confirmation, inference, and maintenance. Adoption is most plausible for projects with many shots, frequent partial brief changes, and distributed teams.

$$
\text{Value per change} = N_{\text{avoided reruns}}(C_{\text{generation}} + C_{\text{review}}) - N_{\text{missed}}C_{\text{re-edit}} - \frac{T_{\text{setup}}\times\text{hourly rate}}{n_{\text{changes}}} - C_{\text{inference}}
$$

All variables are **[DATA NOT AVAILABLE]**. Ask OZU for shots and changes per project, affected-shot share, generation cost per shot, review time per shot, and retry frequency. Also ask who decides whether the team adopts a new tool and whether a budget exists.

| Alternative | Potential advantage | Ripple's proposed difference |
| --- | --- | --- |
| OZU's existing app | Already contains storyboard and prompts | Whether it preserves partial edits across brief changes is unknown (R-08) |
| Manual checklist | No new tool | Does not trace each affected shot automatically |
| Ad hoc AI chat | Fast and cheap | Loses cross-session context and does not retain versions and reasons |

Validate with the three decisive questions at the 20:00 mentor session, then pilot one real project if possible. Commercial expansion beyond OZU is outside this MVP.

## 19. Risks and mitigations

Single-engineer capacity (R-06) and LLM access (R-09) can block the planned demo. Owners below are proposed by role.

| ID | Risk | Likelihood | Impact | Mitigation and check | Proposed owner | Blocks? |
| --- | --- | --- | --- | --- | --- | --- |
| R-01 | Incorrect links cause false negatives or positives | Medium | High | User confirmation, merged-part validation, T-02 | Farras | No |
| R-02 | Director note is ambiguous | High | Medium | Flag ambiguity and request clarification, T-06 | Jo | No |
| R-03 | Manual setup is excessive | Medium | High | Auto-extract points; user corrects; measure setup time | Rasya | No |
| R-04 | Prompt or description is corrupted | Low | High | Deterministic preservation check, T-04 | Farras | No |
| R-05 | Token efficiency claim lacks evidence | Medium | Medium | Measure against B1 and report actual results | Jo | No |
| R-06 | Time runs out | High | High | Prioritize P0, P0b, P1; 03:00 cutoff | Farras | Yes |
| R-07 | Seeded demo overstates live capabilities | Medium | High | Label seeded data and sample media; disclose implementation status | Jo | No |
| R-08 | OZU app already handles partial changes | Unknown | High | Ask at 20:00; if so, emphasize handoff value | Jo | Changes pitch |
| R-09 | LLM access or quota unavailable | Unknown | High | Check early; label seeded fallback | Farras | Yes for AI path |
| R-10 | Higgsfield credentials or credit unavailable | Unknown | High | Validate account/model with one real job; label fallback and make no live-generation claim | Farras | Yes for FR-018 |

## 20. MVP delivery plan

The critical path is M2 → M3 → mentor test → M5, all owned by Farras. The 03:00 cutoff determines which P1 items are dropped. Times below are the original hackathon plan, not a claim about current implementation status. This repository's active backend sequence is in `PLAN.md`; frontend progress lives in `../ripple-fe`.

| Milestone | Time | Owner | Deliverable | Completion evidence |
| --- | --- | --- | --- | --- |
| M1 | 18:30–19:00 | Jo | Section 21 fixture and answer key | Files exist |
| M2 | 18:30–19:15 | Farras | Schema, seeded data, FR-007 | Correct S1 propagation |
| M3 | 19:15–20:00 | Farras | FR-006, FR-008–011 | T-01 passes |
| M4 | 18:30–20:00 | Rasya | Node canvas with S3–S5 panels | BP3 → shot path is traceable |
| M5 | 20:30–22:30 | Farras | FR-002, FR-005 | T-09 passes |
| M6 | 22:30–01:00 | Farras + Rasya | FR-013, S6–S7 | T5 passes internally |
| M7 | 01:00–03:00 | Farras | FR-012, then FR-014 if possible | T-10 passes |
| M7b | After decision flow stabilizes | Farras + Rasya | FR-018 and S8 | Real job reaches terminal state if access exists |
| M8 | 01:00–04:00 | Jo | T-12 versus B1 | Results table |
| M9 | 03:00–08:30 | Farras | Stabilize demo and backup video | Two error-free rehearsals |
| M10 | 07:00–08:30 | Team | Demo and pitch rehearsal | Two passes |

If AI fails, use a clearly labeled sample output and manual path. If automatic links are inaccurate, use confirmed manual links for the demo and report actual accuracy. If time expires, cut FR-014, FR-015, FR-016, then F-14 in that order.

## 21. Synthetic demo specification

The fictional running-shoe campaign **Laju** has 12 shots and three change scenarios. All data is created by the team, not sourced from OZU clients.

| Key | Category | Value | Scope |
| --- | --- | --- | --- |
| BP1 `time_of_day` | timeline | Morning | Global |
| BP2 `color_palette` | color_palette | Warm, golden orange | Global |
| BP3 `logo` | other | Clearly visible on the shoe | Local |
| BP4 `story` | story | A beginner runner builds a routine | Global |

| Shot | Description | Method | Explicitly linked points |
| --- | --- | --- | --- |
| 1 | An alarm rings at dawn; the runner wakes up | AI | BP1 |
| 2 | Close-up of tying shoelaces, visible logo, warm light | Traditional | BP3, BP2 |
| 3 | Wide shot of a neighborhood street in golden morning light | AI | BP1, BP2 |
| 4 | The runner starts jogging slowly | Traditional | BP4 |
| 5 | Shoe sole striking the pavement | AI | None; global points still apply |
| 6 | The runner joins a running community | Traditional | BP4 |
| 7 | Montage of a city waking up | AI | BP1 |
| 8 | The runner stops for water and smiles | Traditional | BP4 |
| 9 | Close-up of the shoe logo during a sprint | AI | BP3 |
| 10 | Step-count graphic on a watch | Traditional | BP4 |
| 11 | The runner reaches a park at sunrise beneath an orange sky | AI | BP1, BP2 |
| 12 | Final shoe packshot with logo and tagline | Traditional | BP3 |

| Scenario | Director note | Diff | Flagged shots | Answer key | Expected metric |
| --- | --- | --- | --- | --- | --- |
| S1 local | “The logo may appear only in the final packshot.” | BP3: clearly visible on the shoe → only in the final packshot | 2, 9, 12 | 2, 9 | Recall 2/2; precision 2/3; producer keeps shot 12 |
| S2 global | “Change to a night run downtown with a cool blue palette and neon lights.” | BP1 morning → night; BP2 warm → cool blue/neon | All 12; text changes in 1, 2, 3, 7, 11; others carry “Applies to every shot; not mentioned directly in the description” | All 12 | Recall 12/12; subject and composition preserved |
| S3 ambiguous | “Make it feel more premium.” | Ambiguous | None | None | Ask for clarification |

The demo sequence is Time (S1, then brief S2) → Cost (S1 regeneration plan: 2 of 12 shots rerun) → People (shot 9 handoff view with versions and reasons) → token ledger and benchmark. Disclose that the storyboard comes from a file, not OZU's app. Label prepared brief points and links if FR-002/005 are incomplete. Show Higgsfield as live only if a real job completes; otherwise label the result as an example. Report tested implementation status for interpretation, propagation, suggestions, decisions, versions, ledger, and generation separately.

## 22. Decision register

The following choices guide the PRD. Open items require team validation; D4 and R-09 matter before enabling AI features.

| ID | Question | Recommendation or current decision | Status |
| --- | --- | --- | --- |
| D1 | Does Ripple create the storyboard? | After a confirmed summary, choose images or video; review a manual or ChatGPT-drafted image plan/storyboard before saving | Confirmed by Farras, 10 October 2026 |
| D2 | Is change input a note or a new brief version? | Director note in P0; new brief version in P1 | Applied; team confirmation pending |
| D3 | Retry thresholds N, M, K? | N=2, K=3; set M from mentor input; editable per project | Open |
| D4 | Stack, AI, storage, hosting? | Go/Chi and PostgreSQL backend; Next.js frontend; ChatGPT/OpenAI for text; Higgsfield for media; R2 for attachments | Stack/providers set by Farras; model access and hosting open |
| D5 | Acceptance thresholds? | 100% recall and preservation on fixture; report actual precision | Applied; team confirmation pending |
| D6 | PDF brief in MVP? | Text P0; PDF P1 | Open |
| D7 | Three-pillar priority? | Time P0; regeneration plan and decisions P0b; retry log P1 | Applied; team confirmation pending |
| D8 | Low-confidence threshold? | Set after reviewing fixture output | Open |
| D9 | PRD owner? | Jo | Open |

## 23. Requirements traceability

| Source | Goal and user need | Requirement | Evidence |
| --- | --- | --- | --- |
| OZU slide 23: shifting direction, stale work | G1/G2; see affected shots and reasons | FR-006/007 | T-02/T-06; S1/S2 Time demo |
| OZU slide 27: update, do not restart; retain versions | G3; preserve work | FR-008/010 | T-04/T-05; diffs and history |
| OZU slide 25: creative decisions remain human | G2; creative control | FR-006/009 | T-06/T-11; refine/keep |
| OZU slides 21/23: retries, handoff | G6/G7; know cost and preserve context | FR-012/013/014 | T-10/T-11; Cost and People demos |
| OZU slide 15: chat loses context | G2; persistent brief context | FR-001–005 | T-08/T-09; brief and storyboard |
| PENS: AI Token Efficiency | G5; measurable use | FR-011 | T-12; ledger and B1 |
| PENS UI/UX; OZU usability | G4; understandable without training | S1–S8 | T-11; mentor test |
| Farras: node-based interface | G2/G4; visible links and impact | FR-017 | T-13; BP3 → shots 2/9/12 |
| Farras: Higgsfield media | G6; generate chosen shot | FR-018 | T-14; AI shot job/result |
| Farras: ChatGPT summary | G4; review brief summary | FR-019 | T-15; confirmed summary |
| Farras: R2 attachments | G3; retain original files | FR-001/020 | T-16; owner retrieval |
| OZU slide 25: public or provided assets | Appropriate demo assets | NFR-05 | Labeled synthetic fixture |

## 24. Adversarial review

An earlier review used one model in five sequential roles, not five independent evaluators. Its findings are reflected above.

| Perspective | Finding and resolution |
| --- | --- |
| Software architect | Ambiguous now means a shot linked to an unconfirmed diff while another diff in the same note is confirmed; this resolves the FR-006/007 propagation conflict. Decision records include `new_version_id` for traceability. Free-text shot descriptions may require many LLM calls; measure rather than overstate token savings. |
| Production worker | Group globally affected shots by reason to reduce review load. |
| Layperson | Explain global-only impact as “Applies to every shot; not mentioned directly in the description.” |
| PENS judge | “2 of 12 rerun” is a synthetic-fixture result and must be labeled as such. |
| OZU agency head | Ask who approves a new tool and whether a budget exists. |

The source To-do tab calls the retry log mandatory, while this PRD places it in P1 and cuts it at 03:00 if necessary. D7 records the unresolved priority decision.

## 25. Implementation handoff

Build the data model and propagation against the section 21 fixture first; the remaining flows depend on them. Highest-priority requirements: FR-007, FR-006, FR-008, FR-009, FR-010, FR-011, FR-004, FR-005, FR-012, FR-013. Minimum model: project, brief, brief point, shot, shot version, change, decision, token log. Add retry and project settings only if FR-014 is built. Frontend specification: project list at `/project`, node canvas within `/project/[projectId]` with S3–S5 for the mentor test, then S6–S8; S1/S2 may use simple forms beside the canvas.

Core propagation: take confirmed point diffs; mark a shot when it has a linked part or a changed global point; mark an otherwise unaffected shot ambiguous only when it links to an unconfirmed point. Propose replacements only for matching affected parts, and verify all other parts remain byte-identical. The graph must distinguish direct links from global applicability.

Required fixture: the Laju brief with four points, 12 shots, S1/S2/S3 notes, and an answer key written before the system runs.

- [ ] T-01 S1 completes end-to-end
- [ ] T-02 S1/S2 reach 100% recall
- [ ] T-04 unaffected parts are byte-identical
- [ ] T-06 S3 requests clarification without propagation
- [ ] T-07 manual path works when AI fails
- [ ] T-11 mentor completes T1–T3
- [ ] T-12 Ripple versus B1 token use is reported
- [ ] P0b regeneration plan and decision log are visible
- [ ] T-13 graph paths and reasons are correct
- [ ] T-14 Higgsfield is live or explicitly labeled as a fallback
- [ ] T-15 summary does not replace the original brief
- [ ] T-16 R2 attachments are owner-protected and retrievable

Resolve ChatGPT/OpenAI API model/access, Higgsfield model/access, and demo hosting (D4, R-09, R-10). Outside this hackathon scope: automatic media-quality scores, automatic method recommendation, vector database, agents, OZU-app integration, and video analysis.
