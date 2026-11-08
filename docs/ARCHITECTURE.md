# Architecture V1 (preview)

## Objectif

Realiser un flux d'installation et d'utilisation de Win32 classiques avec fichiers/registre persistants sans demarrer un second noyau Windows. Le moteur est Sandboxie Plus, appele uniquement via ses interfaces de commande documentees. Il n'y a pas de hook DLL specifique OpenCapsule ni de pilote noyau propre a ce projet en V1.

## Composants

```
OpenCapsule CLI (Go, processus ephemere)
  | manifest JSON / archive ZIP / operations de fichiers
  | SbieIni.exe set + query (configuration avec verification)
  | Start.exe /box:<box> ... (execution et processus)
  v
Sandboxie Plus SbieSvc / pilote de virtualisation (tiers)
  |
  +-- box Files + RegHive persistants dans OpenCapsule/capsules/<id>/box
  +-- processus et applications Win32 dans le bac
  |
  v
Windows hote (noyau partage)
```

## Invariants

1. Un ID valide construit toujours le meme nom de bac `OC_<id>`.
2. Aucun bac pre-existant portant ce nom n'est modifie : `Configure` refuse les collisions.
3. Les options de persistance et le `FileRootPath` sont fixees avant `Enabled=y`.
4. Chaque `set` est suivi d'un `query` et la configuration est verifiee avant les operations.
5. Export/restore/delete verifient que le bac est inactif via `/listpids`. Une reponse vide ou invalide provoque un echec, jamais un « OK » implicite.
6. La V1 n'autorise pas l'elevation directe et configure `DropAdminRights=y`.
7. Les archives ZIP sont limitees, verifiees, et ne peuvent pas ecrire hors du dossier d'extraction.
8. Aucun repertoire etranger n'est efface par un `delete` de capsule valablement geree ; les symlinks sur les dossiers critiques sont refuses.
9. Une restauration s'effectue dans un dossier temporaire, puis par permutation de repertoires avec retour arriere en cas d'erreur de renommage.

## Ce que ces garanties ne couvrent pas

- Les mises a jour de configuration Sandboxie sont non transactionnelles, leurs codes de sortie ne prouvent pas toutes les persistances : la lecture retour reduit le risque sans remplacer un test complet du service.
- Le faux `Idle` via `/listpids` est propre a la session de l'appelant. Un processus dans une autre session ou un demarrage concurrent depuis SandMan peuvent invalider l'hypothese de silence du bac.
- L'overlay Sandboxie et ses registres ne garantissent pas un package portable entre machines.
- Les applications peuvent demander a des services externes des modifications que Sandboxie ne sait pas virtualiser.
- Une couche de pilotes Windows partages ne peut pas etre isolee comme une VM.
- La suppression d'un dossier sur Windows peut echouer en cas de verrouillage par un autre processus. Le logiciel signale l'erreur et ne promet pas un rollback global.

## Evolution du moteur

Une future solution autonome demanderait un moteur d'execution et de virtualisation du registre/fichier independant, couvrant de nombreuses subtilites de NTFS, des processus enfants, des droits et des services Win32. Il faut d'abord etablir une matrice de compatibilite, un banc de test Windows et des fuzzers d'archives/config avant d'envisager de remplacer un moteur mature.
