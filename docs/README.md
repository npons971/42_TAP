# Documentation TAP

La documentation est organisée par responsabilité. Les documents partagés ont une seule source dans `Commun/` ; `Dev_A/Commun` et `Dev_B/Commun` sont des liens symboliques vers ce dossier.

| Tag | Meaning | Action Required |
|---|---|---|
| `🔴 TO_FILL` | **Decision Pending / To Specify** | Team must choose options, define formulas, or provide details |
| `🟡 IN_PROGRESS` | **Work In Progress** | Under active drafting or implementation |
| `🔵 IN_REVIEW` | **Pending Review** | Ready for peer review and mutual alignment |
| `🟢 VALIDATED` | **Approved & Locked** | Finalized baseline specification |



## Dev A — Aris : serveur et CLI

- [Tâches de Dev A](Dev_A/TAP_Aris_Dev_A.md)
- [Architecture serveur](Dev_A/architecture/server_architecture.md) et [modèle de concurrence](Dev_A/architecture/concurrency_model.md)
- [Client CLI](Dev_A/clients/cli_client.md)
- [Logging serveur](Dev_A/logging/server_logging.md)
- [Documentation commune](Dev_A/Commun/TAP_taches_communes.md)

## Dev B — Novanns : GUI et conception du monde

- [Tâches de Dev B](Dev_B/TAP_Novanns_Dev_B.md)
- [Client GUI](Dev_B/clients/gui_client.md)
- [Game Design Document](Dev_B/Commun/gdd/README.md)
- [Documentation commune](Dev_B/Commun/TAP_taches_communes.md)

## Commun — référentiel partagé

- [Tâches et décisions d'équipe](Commun/TAP_taches_communes.md)
- [Index technique et état des documents](Commun/TECHNICAL_INDEX.md)
- [Protocole](Commun/protocol/rfc_syntax.md) et [charges utiles JSON](Commun/protocol/json_payloads.md)
- [Tests d'intégration](Commun/testing/integration_tests.md) et [guide d'évaluation](Commun/testing/peer_evaluation_guide.md)
- [Game Design Document](Commun/gdd/README.md)
