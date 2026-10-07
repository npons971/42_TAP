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

### 1. Protocol Specifications (`Commun/protocol/`)
| Document | Focus | Assigned To | Status |
|---|---|---|---|
| [rfc_syntax.md](protocol/rfc_syntax.md) | TCP framing, 4,096-byte limit, replies, events, IDs | Aris (Dev A) & Novanns (Dev B) | `🔵 IN_REVIEW` — Dev A proposal, Dev B review pending |
| [json_payloads.md](protocol/json_payloads.md) | JSON payloads for `LOOK`, `STATUS`, `WHO`, `INVENTORY`, `QUESTS` | Both | `🔵 IN_REVIEW` — Dev A proposal, Dev B review pending |

### 2. Architecture & Concurrency (`Dev_A/architecture/`)
| Document | Focus | Assigned To | Status |
|---|---|---|---|
| [server_architecture.md](../Dev_A/architecture/server_architecture.md) | Dispatcher vs Router, GameState truth, World loader | Aris (Dev A) | `🔴 TO_FILL` |
| [concurrency_model.md](../Dev_A/architecture/concurrency_model.md) | Goroutines topology, RWMutex, non-blocking broadcasting | Aris (Dev A) | `🟡 IN_PROGRESS` — initial server model implemented |

### 3. Server Logging & Security (`Dev_A/logging/`)
| Document | Focus | Assigned To | Status |
|---|---|---|---|
| [server_logging.md](../Dev_A/logging/server_logging.md) | Structured JSON logs, Audit trails, Flood & abuse detection | Aris (Dev A) | `🔵 IN_REVIEW` |

### 4. Client Specifications (`Dev_A/clients/`, `Dev_B/clients/`)
| Document | Focus | Assigned To | Status |
|---|---|---|---|
| [cli_client.md](../Dev_A/clients/cli_client.md) | Async terminal I/O, Dual goroutine loops, Command interface choice | Aris (Dev A) | `🔵 IN_REVIEW` |
| [gui_client.md](../Dev_B/clients/gui_client.md) | Graphical toolkit selection, Reactive state loop, UI views & controls | Novanns (Dev B) | `🟢 VALIDATED` |

### 5. Testing & Evaluation (`Commun/testing/`)
| Document | Focus | Assigned To | Status |
|---|---|---|---|
| [integration_tests.md](testing/integration_tests.md) | Unit testing, Mock TCP concurrency test harnesses, Flood testing | Both | `🟡 IN_PROGRESS` |
| [peer_evaluation_guide.md](testing/peer_evaluation_guide.md) | Step-by-step 42 peer-evaluation checklist & live code edit preparation | Both | `🔵 IN_REVIEW` |
