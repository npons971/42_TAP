# TAP — Aris (Dev A) : serveur réseau et protocole

Répartition de référence : [organisation à trois](../Commun/TAP_taches_communes.md). Autres pôles : [GUI — Dev B](../Dev_B/TAP_Novanns_Dev_B.md), [moteur, monde et CLI — Dev C](../Dev_C/TAP_Dev_C.md).

## Responsabilités

- Serveur TCP : écoute, connexions simultanées, sessions, files de sortie et arrêt propre.
- Protocole RFC : framing UTF-8, parser, validation syntaxique, routage, réponses OK/ERR et événements EVT pour toutes les commandes.
- Logique des commandes CONNECT, CHAT (GLOBAL/ROOM/GROUP), WHO, GROUP et QUIT.
- Diffusion des événements : destinataires, ordre, déconnexions pendant l'envoi et clients lents.
- Concurrence côté réseau et protection de l'état partagé définie avec Dev C.
- Logs structurés : timestamps, IP, commandes, paramètres, réponses, erreurs et événements métier fournis par le moteur.
- Détection de flooding et de connexions rapides, tests réseau et conformité RFC.
- Documentation : architecture réseau, protocole, logs, lancement du serveur et tests associés.

## Interface avec le moteur

Dev C possède la logique d'exploration, d'objets, de PNJ, de combat et de quêtes. Dev A transmet les commandes au moteur et diffuse leurs résultats. Les deux définissent les structures partagées, les mutations atomiques, le nettoyage à la déconnexion et le contrat d'événements. Le CLI est désormais sous la responsabilité de Dev C.

## Travail existant et passation

Aris a déjà implémenté le serveur et le CLI : commandes obligatoires, objets uniques, combat DEFEND/FLEE, quêtes, groupes, chargement du monde, logs et détection d'abus. Cette contribution historique reste attribuée à Aris ; le suivi du moteur et du CLI est transféré à Dev C.

Les tests réseau, de concurrence et de terminal sont décrits dans [aris_acceptance.md](../Commun/testing/aris_acceptance.md). Le [rapport de conformité RFC](../Commun/protocol/rfc_conformance.md) documente les corrections et extensions. Une session visuelle Wails et l'interopérabilité avec une autre équipe restent des validations communes.

## Références

- [Architecture serveur](architecture/server_architecture.md)
- [Modèle de concurrence](architecture/concurrency_model.md)
- [Logging](logging/server_logging.md)
- [Documentation CLI existante, suivie par Dev C](clients/cli_client.md)

La passation et les jalons de validation sont définis dans le document commun. Les SDK, caches et commandes de build restent locaux au dépôt via le Makefile.
