# Technical Documentation & Progress Dashboard

This document serves as the master index for the technical specifications, architecture decisions, protocol standards, and evaluation guidelines for the **TAP (The Answer Protocol)** project.

---

## Status Tagging System

To track progress across the codebase and identify pending team decisions, every technical document and major decision block uses the following status tags:

| Tag | Meaning | Action Required |
|---|---|---|
| `🔴 TO_FILL` | **Decision Pending / To Specify** | Team must choose options, define formulas, or provide details |
| `🟡 IN_PROGRESS` | **Work In Progress** | Under active drafting or implementation |
| `🔵 IN_REVIEW` | **Pending Review** | Ready for peer review and mutual alignment |
| `🟢 VALIDATED` | **Approved & Locked** | Finalized baseline specification |

---

## Current ownership

[Team organization](TAP_taches_communes.md): Aris owns network/protocol,
Novanns owns GUI, and [Alexis (Dev C)](../Dev_C/TAP_Dev_C.md)
owns the game engine, world and CLI. Shared design, testing and review involve
all three members. Historical document paths and completed contributions are preserved.
Dev C maintains the [shared GDD](gdd/README.md) and the CLI specification
at its historical Dev_A path.

## Documentation Matrix

### 1. Protocol Specifications (`Commun/protocol/`)

External specification: [RFC 42TAP](protocol/external_rfc.html).
[Conformance report and extensions](protocol/rfc_conformance.md):
standard command formats are aligned and checked by dedicated TCP tests.

| Document | Focus | Assigned To | Status |
|---|---|---|---|
| [rfc_syntax.md](protocol/rfc_syntax.md) | TCP framing, 4,096-byte limit, replies, events, IDs | Aris (Dev A), Novanns (Dev B) & Dev C | `🔵 IN_REVIEW` — Dev A proposal, team review pending |
| [json_payloads.md](protocol/json_payloads.md) | Standard JSON payloads and explicit metadata extensions | All three members | `🔵 IN_REVIEW` — Dev A proposal, team review pending |

### 2. Architecture & Concurrency (`Dev_A/architecture/`)
| Document | Focus | Assigned To | Status |
|---|---|---|---|
| [server_architecture.md](../Dev_A/architecture/server_architecture.md) | Dispatcher vs Router, GameState truth, World loader | Aris (Dev A) & Dev C | `🔵 IN_REVIEW` — implemented, ready for team review |
| [concurrency_model.md](../Dev_A/architecture/concurrency_model.md) | Goroutines topology, RWMutex, non-blocking broadcasting | Aris (Dev A) & Dev C | `🔵 IN_REVIEW` — server gameplay implemented |

### 3. Server Logging & Security (`Dev_A/logging/`)
| Document | Focus | Assigned To | Status |
|---|---|---|---|
| [server_logging.md](../Dev_A/logging/server_logging.md) | Structured JSON logs, Audit trails, Flood & abuse detection | Aris (Dev A) | `🔵 IN_REVIEW` |

### 4. Client Specifications (`Dev_A/clients/`, `Dev_B/clients/`)
| Document | Focus | Assigned To | Status |
|---|---|---|---|
| [cli_client.md](../Dev_A/clients/cli_client.md) | Async terminal I/O, Dual goroutine loops, Command interface choice | Dev C | `🔵 IN_REVIEW` |
| [gui_client.md](../Dev_B/clients/gui_client.md) | Graphical toolkit selection, Reactive state loop, UI views & controls | Novanns (Dev B) | `🟢 VALIDATED` |

### 5. Testing & Evaluation (`Commun/testing/`)
| Document | Focus | Assigned To | Status |
|---|---|---|---|
| [integration_tests.md](testing/integration_tests.md) | Unit testing, Mock TCP concurrency test harnesses, Flood testing | All three members | `🟡 IN_PROGRESS` |
| [peer_evaluation_guide.md](testing/peer_evaluation_guide.md) | Step-by-step 42 peer-evaluation checklist & live code edit preparation | All three members | `🔵 IN_REVIEW` |

Server/CLI acceptance matrix: [aris_acceptance.md](testing/aris_acceptance.md).
