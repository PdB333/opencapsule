# Plan de validation sur Windows (a executer avant une release stable)

Aucune execution sur un Windows equipe de Sandboxie Plus n'a ete possible dans l'environnement de construction initial. Cette liste est un protocole, pas un rapport de reussite.

1. Installer Sandboxie Plus (version courante) sur Windows 11 x64, redemarrer si demande.
2. `opencapsule doctor` doit detecter `Start.exe` et `SbieIni.exe`.
3. `opencapsule create --entry C:\\Windows\\System32\\notepad.exe blocnotes` doit creer le bac `OC_blocnotes`, avec `Enabled=y`, `DropAdminRights=y`, `AutoDelete=n`, `FileRootPath` dans LocalAppData.
4. `opencapsule run blocnotes` doit ouvrir Notepad sous Sandboxie (verifier l'indicateur/les processus depuis SandMan), sans copie locale de Windows.
5. Fermer Notepad ; verifier que `opencapsule snapshot blocnotes` marche uniquement quand le bac est arrete.
6. Creer un fichier *dans le bac*, exporter, modifier puis restaurer. Verifier que le fichier hote n'a pas change et que l'overlay est revenu a l'etat attendu.
7. Tester `install` avec un installateur Win32 sans pilote et sans service eleve ; definir le chemin d'entree ; relancer apres reboot et verifier la persistance.
8. Tester `install` avec un `.msi` non privilegie ; documenter les differences de comportement (installateurs MSI complexes non garantis).
9. Tester `export`, `verify`, `import` avec un nouveau nom de capsule ; comparer le contenu apres fermeture et reouverture.
10. Demarrer une application, tenter `snapshot`, `restore` et `delete --yes` : ces commandes doivent refuser l'operation si l'application tourne.
11. Tester les collisions de noms, chemins Unicode, noms invalides, dossiers en lecture seule, root symlink/junction, manque d'espace et archives corrompues.
12. Executer l'ensemble apres une mise a jour Windows et une mise a jour Sandboxie ; surveiller les erreurs de pilote/service.

**Ne pas valider ce logiciel en executant des malwares ni des drivers inconnus sur le Windows principal.**
