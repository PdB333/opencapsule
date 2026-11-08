# OpenCapsule

**Conteneurs persistants pour applications Windows, sans machine virtuelle.**

OpenCapsule est une CLI open source qui transforme les fonctions de sandboxing de **Sandboxie Plus** en un workflow de conteneurs : creation, installation, lancement, sauvegarde, restauration et export/import `.ocap`.

> **Statut : 1.0.0-preview.1.** Le code Go compile en binaire Windows x64 et les tests unitaires et les tests de cycle de vie avec un faux backend passent. **Aucun test de bout en bout sur un PC Windows avec Sandboxie Plus n'a encore ete realise.** Il s'agit d'une preview technique, pas d'une isolation certifiee ou d'un substitut complet a Turbo Studio.

## Pourquoi ce choix ?

Une alternative "from scratch" a Turbo Studio necessiterait de reconstruire la virtualisation des fichiers, du registre, des processus et des services. Pour obtenir une **V1 utilisable** plutot qu'un injecteur fragile, OpenCapsule utilise le moteur de Sandboxie Plus (logiciel open source, installation distincte). Ce projet est **independant** du moteur : il ne redistribue ni ne lie le code Sandboxie. Un backend propre pourra etre developpe dans une version ulterieure.

- **Sans VM** : aucun second Windows ni hyperviseur lance par OpenCapsule.
- **Leger** : binaire CLI natif Go, sans service permanent ajoute. Le moteur Sandboxie apporte ses propres services et pilote.
- **Persistance** : `AutoDelete=n`, fichiers et registre du bac conserves.
- **Gestion** : une capsule = un bac Sandboxie `OC_<id>` avec stockage dans `%LOCALAPPDATA%\OpenCapsule\capsules\<id>\box`.
- **Sauvegardes** : snapshots et packages `.ocap` ZIP compresses, avec SHA-256 par fichier et protections contre les archives malformees.
- **Fail closed pour les operations destructrices** : verification de l'identite du bac et refus de snapshot/restauration si des processus y tournent ou si leur etat ne peut pas etre determine.

## Prerequis

- Windows 10/11 x64 (cible de la preview).
- [Sandboxie Plus](https://sandboxie-plus.com/) installe, service/pilote fonctionnels. Les programmes `Start.exe` et `SbieIni.exe` doivent etre presents dans le meme dossier.
- Pour compiler : Go 1.22 ou plus recent. **Aucun Go requis** pour lancer le `.exe` precompile.
- Certaines fonctions de SandMan/Sandboxie Plus necessitent un certificat de support, mais **OpenCapsule n'active pas ces modes payants**.

## Demarrage rapide (PowerShell)

```powershell
# Placer opencapsule.exe dans un dossier du PATH, puis :
opencapsule doctor
opencapsule create --entry 'C:\Program Files\Example\Example.exe' example
opencapsule install example 'C:\Users\Moi\Downloads\setup.exe'
opencapsule run example

# Si l'executable installe differe de l'exemple :
opencapsule shell example
opencapsule set-entry example 'C:\Program Files\Example\app.exe'
opencapsule run --wait example

# Les sauvegardes n'operent que quand le bac est a l'arret.
opencapsule stop example
opencapsule snapshot example
opencapsule export example 'D:\Backups\example.ocap'
opencapsule verify 'D:\Backups\example.ocap'
opencapsule import example2 'D:\Backups\example.ocap'
opencapsule list
opencapsule inspect example2

# Operation irreversible - conserver une archive externe avant :
opencapsule delete --yes example2
```

Les `install` de fichiers `.msi` lancent `msiexec.exe /i ...` dans le bac : **les MSI qui exigent des services systeme, elevation ou drivers peuvent echouer**. L'option `install --fake-admin` simule un token admin pour les installateurs compatibles ; elle n'accorde pas de veritables privileges eleves.

L'option `shell` ouvre le dialogue Sandboxie « Run Any Program » pour explorer ou lancer une application depuis le bac. Les chemins enregistres par `set-entry` sont des **chemins vus par l'application dans son environnement virtuel**, pas necessairement des fichiers existants sur le Windows hote.

## Commandes

| Commande | Fonction |
| --- | --- |
| `doctor` | Detecter le backend installe. |
| `create [--entry EXE] ID` | Creer un bac persistant. |
| `install [--fake-admin] ID INSTALLER [args...]` | Lancer un installateur dans le bac (et attendre le processus principal). |
| `shell ID` | Ouvrir le dialogue d'execution Sandboxie du bac. |
| `set-entry ID EXE` | Enregistrer le programme a lancer par defaut. |
| `run [--wait] ID [--] [EXE [args...]]` | Executer un programme dans le bac. |
| `stop ID` | Demander la terminaison des processus du bac. |
| `snapshot ID` | Creer une sauvegarde locale du contenu du bac. |
| `export ID FILE.ocap` | Exporter la capsule. |
| `verify FILE.ocap` | Verifier l'archive et ses checksums. |
| `restore ID FILE.ocap` | Remplacer le contenu du bac par un snapshot portant le meme ID. |
| `import NEW_ID FILE.ocap` | Creer une nouvelle capsule depuis une archive. |
| `list`, `inspect ID` | Inventaire des capsules gerees. |
| `delete --yes ID` | Supprimer definition + donnees gerees, si le bac est arrete. |

L'identifiant doit contenir de 2 a 39 caracteres : lettre minuscule initiale, puis lettres minuscules, chiffres, `_`, `-`.

### Configuration

Deux variables d'environnement facultatives :

- `OPENCAPSULE_HOME` : chemin de base du stockage et des manifests (defaut `%LOCALAPPDATA%\OpenCapsule`).
- `OPENCAPSULE_SBIE_DIR` : dossier contenant `Start.exe` et `SbieIni.exe` (sinon recherche dans les dossiers Program Files habituels).

Un verrou `.lock` au niveau du dossier de base protege les mutations simultanees faites par **OpenCapsule** ; il ne bloque pas des operations concurrentes dans SandMan. En cas de crash et de verrou obsolete, verifier qu'aucune commande n'est active avant de supprimer manuellement le fichier `.lock`.

## Format `.ocap`

Le package est un ZIP versionne :

```text
manifest.json
checksums.json
\- data/
   \- drive/... (fichiers du bac Sandboxie)
   \- RegHive (si present)
```

L'export refuse les liens symboliques/jonctions, limite la taille decompressee totale a 8 Gio et refuse les chemins `..`, les noms en conflit de casse et les fichiers non reguliers. Les checksums detectent les corruptions **accidentelles** ; ils ne constituent pas une signature cryptographique d'editeur. Les archives peuvent contenir des **donnees personnelles, cookies ou secrets d'application** : les traiter comme des sauvegardes confidentielles.

**Portabilite non garantie :** une ruche Sandboxie, les dependances et les licences d'un programme peuvent etre liees a une version Windows, un profil utilisateur ou une architecture particuliere. Tester un import sur le PC cible.

## Limites de securite importantes

OpenCapsule ne constitue **pas** une frontiere de securite fiable contre un pilote noyau hostile. Les drivers, services SYSTEM externes, installateurs demandant de vrais droits administrateur et applications avec des acces hors sandbox ne sont **pas** pris en charge. Les processus dans un bac standard peuvent lire beaucoup de fichiers hote ; **ne pas utiliser cette preview pour executer des malwares ou proteger des secrets contre un logiciel malveillant**.

Le moteur Sandboxie est installe sur l'hote et dispose de son propre pilote : « sans VM » ne signifie **pas** « sans pilote ». Les services et pilotes instables ou malveillants ne doivent pas etre installes sur l'hote en guise de contournement.

Les snapshots ne sont pas transactionnels vis-a-vis de SandMan ou des autres outils ; ils exigent un bac inactif et peuvent echouer si une action externe le relance pendant la copie. Ne pas utiliser OpenCapsule comme unique mecanisme de sauvegarde.

## Compilation et tests

```powershell
go test ./...
go vet ./...
go build -trimpath -ldflags='-s -w' -o opencapsule.exe ./cmd/opencapsule
```

Depuis Linux/macOS, une compilation croisee est possible :

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o opencapsule.exe ./cmd/opencapsule
```

Le workflow `.github/workflows/ci.yml` lance les tests sur Linux et Windows, effectue une verification statique et construit un executable Windows x64 publie comme **artifact CI**, pas comme release certifiee.

## Documentation et feuille de route

- [Architecture et garanties](docs/ARCHITECTURE.md)
- [Validation Windows](docs/WINDOWS-TEST-PLAN.md)
- [Fonctionnalites suivantes](docs/ROADMAP.md)
- [Securite](SECURITY.md)

Licence du code OpenCapsule : **MIT**. Sandboxie Plus reste un projet tiers avec sa propre licence (principalement GPLv3) : ce projet **ne vend pas, ne modifie pas et n'integre pas** ses binaires ou son code source.
