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

## Documentation Matrix

### 1. Protocol Specifications (`docs/protocol/`)
| Document | Focus | Assigned To | Status |
|---|---|---|---|
| [rfc_syntax.md](file:///home/almarc/42/42_TAP/docs/protocol/rfc_syntax.md) | TCP Framing, ABNF Grammar, Request/Reply lifecycle, Event definitions | Aris (Dev A) | `🔵 IN_REVIEW` |
| [json_payloads.md](file:///home/almarc/42/42_TAP/docs/protocol/json_payloads.md) | Exact JSON payloads for `LOOK`, `STATUS`, `WHO`, `INVENTORY`, `QUESTS` | Both | `🔴 TO_FILL` |

### 2. Architecture & Concurrency (`docs/architecture/`)
| Document | Focus | Assigned To | Status |
|---|---|---|---|
| [server_architecture.md](file:///home/almarc/42/42_TAP/docs/architecture/server_architecture.md) | Dispatcher vs Router, GameState truth, World loader | Aris (Dev A) | `🔴 TO_FILL` |
| [concurrency_model.md](file:///home/almarc/42/42_TAP/docs/architecture/concurrency_model.md) | Goroutines topology, RWMutex vs Channels, Non-blocking broadcasting | Aris (Dev A) | `🔴 TO_FILL` |

### 3. Server Logging & Security (`docs/logging/`)
| Document | Focus | Assigned To | Status |
|---|---|---|---|
| [server_logging.md](file:///home/almarc/42/42_TAP/docs/logging/server_logging.md) | Structured JSON logs, Audit trails, Flood & abuse detection | Aris (Dev A) | `🔵 IN_REVIEW` |

### 4. Client Specifications (`docs/clients/`)
| Document | Focus | Assigned To | Status |
|---|---|---|---|
| [cli_client.md](file:///home/almarc/42/42_TAP/docs/clients/cli_client.md) | Async terminal I/O, Dual goroutine loops, Command interface choice | Aris (Dev A) | `🔵 IN_REVIEW` |
| [gui_client.md](file:///home/almarc/42/42_TAP/docs/clients/gui_client.md) | Graphical toolkit selection, Reactive state loop, UI views & controls | Novanns (Dev B) | `🔴 TO_FILL` |

### 5. Testing & Evaluation (`docs/testing/`)
| Document | Focus | Assigned To | Status |
|---|---|---|---|
| [integration_tests.md](file:///home/almarc/42/42_TAP/docs/testing/integration_tests.md) | Unit testing, Mock TCP concurrency test harnesses, Flood testing | Both | `🟡 IN_PROGRESS` |
| [peer_evaluation_guide.md](file:///home/almarc/42/42_TAP/docs/testing/peer_evaluation_guide.md) | Step-by-step 42 peer-evaluation checklist & live code edit preparation | Both | `🔵 IN_REVIEW` |
