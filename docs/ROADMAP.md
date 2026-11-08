# Roadmap

## V1 Preview — realise dans ce depot

- CLI Windows x64 sans service OpenCapsule permanent.
- Definition et verification d'un bac Sandboxie Plus, donnees persistantes.
- Installateur EXE, MSI avec limitations, mode fake-admin optionnel.
- Execution GUI et gestion de l'application principale.
- Package .ocap verifie SHA-256, export/import, snapshots/restauration.
- Refus des archives ZIP malicieuses (path traversal, symlinks, gros volumes).
- CI multiplateforme, compilation croisee Windows, tests du cycle de vie sur faux backend.

## V1 stable — criteres de sortie

- Matrice de tests manuels sur Windows 10/11 x64 et versions supportees de Sandboxie Plus.
- Verification des installations EXE/MSI et des applications lancees depuis leur chemin virtuel.
- Tests longue duree (apps GUI, mise a jour de l'hote, plusieurs sessions et utilisateurs).
- Tests de regression sur chemins longs, jonctions/reparse points NTFS, montages reseau.
- Verification des transactions de snapshot et du cycle de vie SandMan avec une autre instance active.
- Installer MSI pour OpenCapsule, signature du binaire, documentation de la provenance.

## V2 envisagable

- Interface graphique native (Qt, WinUI ou webview embarquee).
- Profiles d'application et import facile des raccourcis.
- Isolation reseau et permissions configurables avec avertissements clairs.
- Nettoyage/repair d'un bac partiellement configure.
- Mises a jour et versionnement reproductible des archives.
- Backend autonome experimental par API Windows documentees (sans promesse de parite Turbo).

## Hors objectif

- Contenir un driver kernel dans un bac partageant le noyau.
- Executer du code de noyau ou un service SYSTEM non fiable dans une isolation garantie sans VM.
- Presenter cette preview comme une sandbox anti-malware certifiee.
