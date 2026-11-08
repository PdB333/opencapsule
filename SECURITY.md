# Security policy

OpenCapsule est une V1 de gestion de conteneurs applicatifs persistants **sans VM**, mais **pas** un mecanisme garanti contre des logiciels malveillants.

- Le moteur tiers Sandboxie Plus fonctionne sur le **noyau Windows partage** et dispose d'un pilote dans le systeme hote.
- Les drivers noyau tiers, services SYSTEM et elevations d'installateur non prises en charge ne sont pas isolables comme des applications Win32 ordinaires.
- Les bacs standards peuvent lire des fichiers de l'hote ; ne pas y exposer de secrets si le logiciel est potentiellement malveillant.
- L'outil ne modifie pas directement le registre Windows de l'hote. Il appelle uniquement les API CLI documentees de Sandboxie. `SbieIni` peut laisser une configuration partielle apres un echec ; **examiner SandMan** si une creation echoue.
- Les archives `.ocap` ne sont **pas chiffrees**. Elles peuvent contenir des tokens, cookies, mots de passe ou fichiers personnels. Conserver les exports sur un volume chiffre et ne pas les publier.
- Le verificateur de checksums protege contre une corruption de fichiers, **pas** contre une archive volontairement modifiee et resignee par un attaquant.
- Le verrou local protege contre des operations OpenCapsule concurrentes, pas contre SandMan ou d'autres utilitaires.
- Pour signaler une vulnerabilite : ouvrir une alerte de securite privee GitHub sur le futur depot, ou contacter le mainteneur hors d'un ticket public. Ne pas poster de secrets ou d'archives d'utilisateurs dans une issue publique.
