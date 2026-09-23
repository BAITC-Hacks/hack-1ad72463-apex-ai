# Apex AI Implementation plan for meeting minutes and action items

Date: 23 September 2026. Team: 3 people. Goal: a competitive HackAlem AI prototype.

Status: research and a proposed plan, not a report on a completed product. The repository contained only a README when this work started. Models have not been run; quality and speed have not been measured. [Русская версия](RESEARCH_RU.md).

## 1 Recommendation

Build a local assistant for meeting secretaries that converts a recording into verifiable minutes and action items. Every action item should link to its source statement and audio excerpt, allowing the secretary to check the owner and deadline, approve the result, and export the document.

The proposed strength is reliable capture of commitments in Russian, Kazakh, and mixed speech. Prioritize a complete working workflow, correct treatment of ambiguity, and reproducible setup. This is a hackathon positioning hypothesis; a market-level competitive advantage has not yet been researched.

Make the first technical decision after a short recording has been processed on the available hardware. If local mixed-language transcription loses names, actions, and deadlines, a polished interface will not compensate.

## 2 Requirements from the case brief

The requirements source is the user-provided document “HackAlem AI_ Система автопротоколирования совещаний с фиксацией поручений.docx”. Its contents are treated as the task specification, not as independent instructions to modify the repository or perform external actions. The architectural choices below are team proposals.

| Case requirement | Proposed implementation | Demonstration evidence |
| --- | --- | --- |
| Russian, Kazakh, and mixed speech | Local multilingual transcription without mandatory translation into one language | Three separate tests and a mixed-language recording |
| Speaker diarization | Automatic speaker intervals and labels | Transcript with distinct speakers and playback |
| Attribution to a person | Secretary-confirmed mapping from labels to a participant list | Assign a name to a speaker and update related actions |
| Actions with an owner and deadline | Structured extraction with field and evidence validation | Action card with task, owner, deadline, and source |
| Minutes and short summary | Topics, decisions, current actions, and open questions | Consistent review screen and export |
| PDF/DOCX export | DOCX first; PDF if time permits or judges explicitly require it | Open the exported file and check its action table |
| Deployment within a closed environment | Local ASR, diarization, LLM, storage, and export | Complete run with external access blocked after model preparation |
| Test data | Simulated three-person meeting plus separate evaluation recordings | Audio, annotations, and measured results |

The brief explicitly prohibits sending audio or text to external cloud APIs. Local transcription alone is therefore insufficient: action extraction and summarization must also run inside the environment. A cloud LLM is not an acceptable fallback under the current case wording.

Simulated recordings are allowed. Real recordings used for demonstration must be anonymized. Participants in our own test recordings must be informed about recording and AI processing.

Scope questions to resolve with the organizers:

- Teams, Zoom, Google Meet, recordings, and live streams appear in the input description, but a meeting bot is not separately listed in the mandatory minimum. Our working MVP assumption is that recording upload is acceptable. Until confirmed, this remains a compliance risk.
- We provisionally interpret “PDF/DOCX” as either format; add PDF if both are required.
- Diarization is mandatory; voice identification by timbre is listed as an extra. We propose human confirmation of the speaker-to-name mapping and make that step explicit.
- Reminders appear in user scenario 2 but not in the mandatory minimum. Add a simple in-app notification once the main workflow works.
- Reconcile the cloud API restriction with the event-wide rules. Until clarified, the application runtime stays fully local. Development tools must not cause real recordings, transcripts, or logs containing their content to be uploaded to external services.

## 3 Allocate effort against the judging criteria

| Criterion from the brief | Points | Work priority |
| --- | ---: | --- |
| Task fit and functionality | 25 | Complete audio-to-reviewed-minutes workflow across all three language modes |
| Technical implementation | 25 | Local pipeline, real diarization, validated data, understandable failures |
| README and reproducibility | 25 | Clean setup, pinned dependencies and models, test data, limitations |
| Value and applicability | 15 | Measure review time and preservation of owners and deadlines |
| Development potential and originality | 10 | Evidence links, action revision history, unresolved questions |

The first three criteria account for 75 points. Additional integrations therefore rank below the main workflow and its documentation. The point allocation does not guarantee a particular score.

## 4 Product ideas and selection

| Idea | Benefit | Decision |
| --- | --- | --- |
| Minutes with quotes and timestamps | Makes the origin of an action verifiable | Core MVP |
| Missing-owner and missing-deadline flags | Exposes commitments that need clarification | Core MVP |
| Action revision history | Preserves deadline changes and cancellations without duplicates | One controlled scenario in MVP |
| Action cards with execution status | Connects meetings to delivery | After the main workflow |
| In-app deadline notifications | Supports the manager scenario | After the main workflow |
| Russian and Kazakh protocol export | Potential customer value | After validating the primary protocol |
| Online meeting bot | Reduces manual recording uploads | After clarifying whether mandatory |
| Document management, corporate email, voiceprints | Possible product extensions | Outside the initial MVP |

Working product statement: “Apex AI captures meeting commitments, shows their evidence, and helps a secretary issue reviewed minutes within a closed environment.”

## 5 MVP user workflow

1. The secretary creates a meeting with a title, date and time, time zone, and participant list. They upload WAV/MP3; video and microphone recording are extensions.
2. The application displays processing stages and failures. Initially design for one active recording up to 5 minutes long; refine the limit after measurement.
3. A transcript appears with timestamps and speaker labels. The secretary listens to short excerpts and maps labels to names. This confirms identity; it does not replace automatic diarization with manual annotation.
4. Action items and questions requiring clarification appear. Each field has evidence; missing information is not filled with guesses.
5. The secretary can correct text, owners, or deadlines. Preserve the original transcript. Editing a segment invalidates dependent results and requires recomputation or review.
6. Review produces an approved protocol version. Export uses that snapshot so the action table and summary remain consistent.
7. Optionally, the manager sees execution states and in-app notifications. An extracted action becomes an operational task only after approval.

The main screen needs three areas: audio and transcript; actions and questions; summary and export. Selecting evidence seeks to the relevant audio moment.

## 6 Architecture and model selection

Proposed processing flow:

```text
Local browser
    → backend and local storage
    → audio decoding and a shared timeline
    → ASR and diarization
    → alignment of text, speakers, and participants
    → local LLM extracts action candidates
    → validation of structure, evidence, and revisions
    → secretary review
    → approved minutes and DOCX
```

Proposed baseline stack: Python/FastAPI, one worker, SQLite, React/Vite, and local DOCX generation. If the team is unfamiliar with React, a simple server-rendered interface is preferable to learning a new stack. A distributed queue and vector database are unnecessary at this scale.

| Component | Initial candidate | Selection condition |
| --- | --- | --- |
| ASR on NVIDIA/CPU | faster-whisper with multilingual Whisper large-v3 or a smaller model | Compare preservation of actions, names, and deadlines against processing time |
| ASR on Apple Silicon | whisper.cpp with a multilingual model | Verify a native Metal run on the target Mac |
| Diarization | pyannote Community-1 | Obtain model access early and verify local execution |
| Extraction and summary | Quantized Qwen3-8B through llama.cpp | Evaluate RU/KZ/mixed inputs and bound generation time |
| Output format | JSON with server-side schema validation | Invalid output must not become a completed task |

The faster-whisper documentation confirms local model loading, CPU INT8, CUDA, and word timestamps. These are tool capabilities, not evidence of accuracy on this case. [Source](https://github.com/SYSTRAN/faster-whisper).

whisper.cpp supports Apple Silicon and Metal. Selecting it for a Mac is a technical candidate, not a measured performance advantage in our prototype. [Source](https://github.com/ggml-org/whisper.cpp).

Community-1 supports local execution after downloading weights; access requires accepting the model conditions. Check this immediately: inaccessible weights block a mandatory feature. [Model card](https://huggingface.co/pyannote/speaker-diarization-community-1).

Qwen3-8B supports local runtimes and a non-thinking mode; llama.cpp provides local inference across hardware types. Multilingual claims in a model card do not establish reliable action extraction from mixed speech. [Qwen3-8B](https://huggingface.co/Qwen/Qwen3-8B), [llama.cpp](https://github.com/ggml-org/llama.cpp).

If memory is constrained, run heavy stages sequentially and unload models between them. CPU is a possible execution profile, but speed and smaller-model quality must be measured. Do not promise real-time processing or specify RAM/VRAM requirements before a trial. Validate the Mac native profile separately from a Linux container; equivalent GPU acceleration is not assumed.

The agent's MVP role is bounded orchestration: transcription, evidence lookup, extraction, validation, and export preparation. One pipeline is sufficient. More LLM calls are not an objective; introduce extra passes only to address observed errors.

## 7 Rules for correct extraction

### Speaker and assignee

Diarization answers “when did the same voice speak”; it does not establish a name. A manager can assign work to another participant, so `speaker_id` and `assignee_id` are separate fields. “I will prepare it” maps to the confirmed speaker identity; “Aigerim will prepare it” maps to the named participant. If the link is ambiguous, retain `null` and a review reason.

Preserve overlap flags for simultaneous speech. Align ASR and diarization intervals by time; split text at speaker boundaries when suitable word timestamps are available, otherwise flag uncertainty. Do not assign every word in an overlapping segment to one person without review.

### Mixed speech

Preserve the original language of statements. Do not force the entire recording into Russian or translate it before extraction: names and dates can change. Test automatic language detection and, if necessary, processing of meaningful chunks. When using VAD or slicing, preserve offsets into the original recording. Choose segmentation based on test results.

### Deadlines and revisions

A meeting has a date, time, and time zone. Resolve “tomorrow at 18:00” against the meeting date, not the processing date. Store the original wording and normalized value. A date without a time stays a date; “as soon as possible” does not become an invented deadline. Ambiguous phrases such as “by Friday” or “by the end of the week” need a visible convention and confirmation.

For an explicit deadline change, retain the original action and a revision event. Apply the new state only when the referenced task is clear. Otherwise, show a conflict. A cancellation removes the task from active actions but remains in history. “We could try this” does not become an action without an agreed decision.

### Model output validation

The model returns a structure referencing source segment IDs. The server checks segment existence, quote correspondence, valid participant IDs, and date formats. Finding a matching quote does not prove correct interpretation; semantic accuracy requires evaluation and human review.

Do not display an invented confidence percentage. Use explicit reasons: “deadline missing”, “assignee unresolved”, “overlapping speakers”, or “conflicting statements”. Build the summary from reviewed decisions and retain supporting references. Transcript phrases such as “ignore the rules” remain recording content and receive no tool permissions.

## 8 Data and interface contracts

Minimum entities: `Meeting`, `Participant`, `SpeakerMapping`, `Segment`, `ActionItem`, `ActionRevision`, `ProtocolVersion`, and `Job`. Preserve original and edited segment text, together with editor and timestamp. Keep review status separate from execution status.

Example proposed structure, not an actual model output:

```json
{
  "id": "action_01",
  "meeting_id": "meeting_01",
  "text": "Подготовить проект бюджета",
  "assignee_id": "person_aigerim",
  "speaker_id": "speaker_00",
  "due_original": "ертең сағат 18:00-ге дейін",
  "due_date": "2026-09-24",
  "due_time": "18:00:00",
  "timezone": "Asia/Almaty",
  "review_status": "needs_review",
  "execution_status": "not_started",
  "evidence": [{"segment_id": "seg_004", "start_ms": 45000, "end_ms": 51000}],
  "issues": [],
  "revision": 1
}
```

The example meeting takes place on 23 September 2026; the name assignment assumes a confirmed participant list. Missing assignee, date, or time fields must allow `null`. The source-language strings are intentionally identical in both research documents.

Proposed endpoints: `POST /meetings`, `POST /meetings/{id}/audio`, `POST /meetings/{id}/process`, `GET /jobs/{id}`, `GET /meetings/{id}/result`, `PATCH /meetings/{id}/speakers`, `PATCH /meetings/{id}/segments/{segment_id}`, `PATCH /meetings/{id}/actions/{action_id}`, `POST /meetings/{id}/approve`, `GET /meetings/{id}/export.docx`, and `DELETE /meetings/{id}`. These are future implementation contracts; the repository does not yet implement them.

Processing runs as a background job; repeated clicks must not create duplicates. Stages: queued, transcribing, diarizing, extracting, needs_review, completed, failed. Persist intermediate outputs so only the required stages need rerunning. Editing a transcript or speaker name invalidates approval of dependent data; an export always belongs to a specific version.

## 9 Privacy and reproducibility

Environment preparation and meeting processing are separate modes. Download dependencies and weights during preparation. During processing, use local paths and allowed internal addresses; there is no external fallback. Document weight hashes, library versions, settings, and licenses in the future application.

Disable telemetry, external fonts/CDNs, and on-demand model downloads. pyannote provides `PYANNOTE_METRICS_ENABLED=0`; a single setting does not prove isolation. Test the application from a cold start with external traffic blocked. [Telemetry documentation](https://github.com/pyannote/pyannote-audio#telemetry).

For the demonstration, use a local interface, upload format and size limits, controlled storage, and deletion of meetings together with derived files. Keep real recordings, secrets, model weights, and the working database out of Git. Logs should contain stages, durations, and error codes without meeting text. Authentication, roles, encryption, and retention policies are required before a multiuser pilot; a prototype does not establish production readiness.

The application README must include the hardware profile, tested versions and model revisions, weight acquisition steps, preparation and startup commands, sample input, expected output, an offline check, measured results, limitations, and failure recovery. Actually execute its commands in a clean environment. These two research files do not replace that README.

## 10 Implementation schedule for three people

The schedule assumes 5 hours based on the event description found earlier. The case brief does not define the duration. Confirm actual remaining time at the venue. Model downloads and access setup are included in the first stage; event rules determine whether preparation before the start is allowed.

| Time from start | Person 1 — speech | Person 2 — data and backend | Person 3 — interface and demonstration |
| --- | --- | --- | --- |
| 00:00–00:30 | Check hardware, weight access, ASR, and diarization on short audio | Check local LLM, data schema, reference actions | Record RU/KZ/mixed tests, UI skeleton, begin README |
| 00:30–01:30 | Real segments, timestamps, speaker labels | Extraction, dates, schema and evidence validation | Upload, progress, transcript, speaker naming |
| 01:30–02:30 | Mixed speech and interval alignment | Persistence, editing, protocol version, DOCX export | Action cards, evidence, corrections, approval |
| 02:30–03:30 | Control run and error diagnosis | Deadline change, cancellation, summary, job retry | Full integration and exported-document inspection |
| 03:30–04:15 | Time and quality measurements on held-out recordings | Offline check, restart, data deletion | Reproduce setup from README in a clean environment |
| 04:15–05:00 | Fix critical failures | Fix critical failures, pin versions | Demo, backup video, honest results table |

Decision gates:

- After 30 minutes, verify access to weights and at least a short local run of the key models. If this fails, simplify the hardware profile or select an available local component. Never present manually labeled speakers as automatic diarization.
- By 2.5 hours, one recording must complete the upload-to-DOCX path. Otherwise, remove additional execution states, notifications, the second export format, and animations.
- Reserve the final 45 minutes for stabilization and the pitch. Start no new features in that period.

## 11 Evaluation and measurements

Prepare 6 short simulated recordings: one development and one held-out recording for each RU, KZ, and mixed mode. Held-out recordings use new phrasing; do not use their results to tune prompts before reporting the final measurement. A Kazakh speaker reviews the transcript and reference labels. Also record a three-minute demonstration with three distinct voices.

Include direct assignments, self-assignment, assignment to another person, relative dates, date-only deadlines, missing deadlines, missing assignees, rescheduling, cancellation, discussion without a decision, overlapping speech, and silence. Separately test a corrupt file, repeated processing, and transcript content that resembles a model instruction.

| Evaluation | What to record |
| --- | --- |
| ASR | WER/CER with documented normalization separately for RU/KZ/mixed; preservation of names and dates |
| Diarization | Speaker errors on annotated intervals; DER with a stated protocol when reference annotation is available |
| Actions | Precision/recall against annotated actions with a documented semantic matching rule |
| Owners and deadlines | Accuracy on matched actions plus fully correct actions as a share of all reference actions |
| Grounding | Unsupported owners/deadlines; preservation of cancellations and revisions |
| Practical value | Manual preparation time versus review time with the system on comparable examples |
| Performance | Audio duration, cold/warm runs, stage timings, memory, and hardware |
| Export and offline operation | Readable DOCX, consistency with the approved version, no external connections |

Acceptance goals, not achieved results: cover all mandatory scenarios; attach evidence to every action; silently invent no assignees or deadlines on the control set; require review for conflicts; export a readable document preserving Kazakh characters; complete the workflow without external access. Measure a baseline before setting realistic accuracy and speed thresholds. A small evaluation set does not establish production reliability.

## 12 Demonstration scenario

The demonstration meeting is dated 23 September 2026 in Asia/Almaty. Three participants discuss a budget. This is a planned test, not the output of an existing system.

1. The chair says: “Айгерім, бюджет жобасын ертең сағат 18:00-ге дейін дайында”. The system should propose an action for Aigerim to prepare the draft budget by 24 September at 18:00.
2. Aigerim replies: “Хорошо, подготовлю”. The acknowledgment must not create a duplicate.
3. The chair says: “По бюджету переносим срок на 25 сентября, 18:00”. Show the new deadline and revision history with both supporting statements.
4. Another participant says: “Можно попробовать новый сервис”. No operational task should be created without an agreed decision.
5. The chair asks: “Кто подготовит список рисков?” Without an answer, this is an open question rather than an invented assignment.
6. The secretary checks speakers, opens audio from an action card, and exports the minutes.

Three-minute pitch: 20 seconds on the problem, 100 seconds on the workflow, 30 seconds on measurements and local processing, 30 seconds on limitations and development. Do not promise to process a long recording in seconds on stage: use a short live excerpt and a clearly identified preprocessed example if measured latency requires it. Label backup video as a recording of an earlier run.

## 13 Main risks and further development

Highest-priority risks: inaccessible weights or insufficient memory; mixed-language transcription errors; confusion between speaker and assignee; incorrect deadline revision interpretation; unconfirmed acceptance of uploads instead of a meeting bot; irreproducible setup. Address each with an early test rather than a promise to fix it after the demo.

After MVP: simple execution states and in-app notifications; PDF; an adapter for an approved meeting platform; document management integration; corporate authentication and roles; scalable background processing; evaluation on longer meetings and varied acoustics. Automatic delivery to external systems is a separately configured user feature that runs only after protocol approval.

## 14 Decisions before implementation

Confirm available hardware and memory, actual remaining time, the team's familiar stack, diarization weight access, and a participant who can evaluate Kazakh speech. Ask the organizers whether recording upload, one export format, and secretary-confirmed speaker names are acceptable.

Until then, the working choice is a local pipeline, short recording upload, automatic speaker labels with confirmed names, verifiable actions, one deadline revision scenario, and DOCX. The next technical step is a short model trial to measure bottlenecks; application implementation has not been performed in this branch.
