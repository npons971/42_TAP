# Alignement avec le RFC externe 42TAP

Source : [RFC fourni dans `rfc.tar.gz`](external_rfc.html), December 2024.
La copie HTML est identique au contenu de l'archive. Migration réalisée le
2026-10-07 ; le [contrat implémenté](rfc_syntax.md) décrit les extensions.

## Corrections réalisées

| Sujet | Contrat actuel |
|---|---|
| Salutation | `OK hello proto=1` |
| Erreurs | Codes numériques sur trois chiffres, token standard et explication spécifique |
| Casse des commandes | Verbes insensibles à la casse, ainsi que scopes CHAT et actions GROUP |
| QUIT | `OK bye` avant EOF, avec nettoyage de session |
| WHO | `OK players=<nombre>` |
| LOOK | Tableaux d'IDs pour items et npcs |
| INVENTORY | Tableau d'IDs |
| TALK | Texte de dialogue sur une ligne |
| STATUS | hp, max_hp, status, plus détails de combat documentés |
| ATTACK | attacker_hp, target_hp, damage, status, plus détails documentés |
| GROUP CREATE | Aucun argument, ID généré, `OK group=<id>` |
| GROUP INVITE | Invitation d'un joueur connecté, événement privé INVITE |
| GROUP JOIN | Cible le pseudo du chef connecté, `OK group=<id>` |
| Événements GROUP | INVITE, JOIN, LEAVE et CHAT ; LEADER en extension |
| Événements STATS | Comptage actualisé après connexion et déconnexion |
| QUEST | Accepte une quête auprès d'un PNJ local par ID ou nom complet |
| QUESTS | Quêtes acceptées/terminées du joueur, quest_id/status/progress |
| Unicode | Pseudos en lettres Unicode minuscules, messages UTF-8 |
| Limites | 128 connexions TCP, 32 objets en inventaire, limiter de commandes, files bornées |
| Clients | CLI compatible, GUI adapté avec extensions explicites et corrélation des réponses |

Le serveur conserve les objets uniques, la défense/fuite, la mort avec retour
à 30 PV, la progression des quêtes et les récompenses globales uniques. Ces
mécaniques sont laissées aux implémenteurs par le RFC et justifiées dans README.

## Choix explicitement documentés

- 4 096 octets par ligne, LF compris, au lieu des 1 024 recommandés : permet
  les descriptions/dialogues détaillés. Les dépassements sont rejetés sans
  tronquer le JSON ni valider une action dont la réponse ne peut être envoyée.
- LF suit les sections transport et grammaire générale ; la production CONNECT
  mentionnant CRLF est contradictoire avec ces sections.
- Pseudos minuscules à la demande d'Aris : 3–20 codepoints, lettres minuscules,
  chiffres décimaux et underscore. Les IDs du monde restent en ASCII minuscule.
- Les codes d'erreur standard sont suivis d'un conseil propre à la commande.
  Des codes/tokens supplémentaires couvrent les erreurs non définies par le RFC.
- Les invitations sont informatives ; le RFC n'impose pas l'adhésion uniquement
  sur invitation. Le chef sortant est remplacé par le premier membre restant
  dans l'ordre des pseudos ; un événement LEADER annonce ce changement.
- Le GUI utilise LOOK DETAILS, INVENTORY DETAILS et TALKJSON pour les noms et
  attributs. WHO DETAILS, QUESTINFO et QUESTS DETAILS restent des extensions
  explicites ; les commandes standard conservent leurs formats RFC.
- Une quête déjà acceptée dont la récompense unique est attribuée à un autre
  joueur reste affichée avec statut d'extension unavailable. Les quêtes non
  acceptées ne sont pas listées par QUESTS.

## Vérification

Les tests de [rfc_test.go](../../../internal/server/rfc_test.go) lisent les
trames TCP sans filtrer les événements STATS. Ils vérifient notamment les
formats standard, les codes d'erreur, l'ordre réponse/événement/EOF, le flux
fragmenté au milieu d'un caractère UTF-8, les commandes regroupées, les
pseudos Unicode, les groupes et les quêtes par PNJ.

Les tests existants conservent la couverture des objets, du combat, des
récompenses concurrentes et des échecs atomiques ; les assertions de
métadonnées utilisent maintenant explicitement les extensions DETAILS/TALKJSON.
Les saturations de connexion/inventaire et les livraisons à capacité sont
également vérifiées. Le backend GUI dispose d’un test TCP et de déconnexion. Les handlers réels du
frontend sont testés pour WHO, listes vides, IDs de quêtes, événements, erreurs
CONNECT et repli sur les commandes standard.

La compilation du frontend et les tests backend ne remplacent pas une session
visuelle du GUI Wails. Une connexion avec le client indépendant d'une autre
équipe reste une vérification d'interopérabilité externe à réaliser.
