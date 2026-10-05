# Feature: Instalador multi-SO (macOS, Linux, Windows) + PATH

## Description
Scripts de instalación que compilan tcode a **`~/.tcode/bin`** (la carpeta
`.tcode` del usuario, que ya es el home de config: se agrega `bin/` sin tocar
`config.json`/`theme.json`) y agregan esa carpeta al **PATH del usuario** para
poder ejecutar `tcode` desde cualquier terminal.

Requisito: Go 1.25+ (el `go.mod` declara 1.25.8). Los binarios no se
distribuyen: se compilan en la máquina destino (el `.gitignore` ya excluye
`*.exe` y `/bin/`).

## Tasks
- [ ] `scripts/install.sh` (bash, macOS+Linux): compila a `~/.tcode/bin/tcode`;
  agrega el bloque marcado `# >>> tcode >>>` con `export PATH="$HOME/.tcode/bin:$PATH"`
  a `~/.zshrc` y `~/.bashrc` (creándolos si faltan, idempotente); `--uninstall`
  borra `bin/` y el bloque; mensaje de terminal nueva para el PATH <!-- id: 0 -->
- [ ] `scripts/install.ps1` (Windows): compila a `~\.tcode\bin\tcode.exe`;
  agrega el bin al PATH **de usuario** vía `[Environment]::SetEnvironmentVariable('Path',...,'User')`
  (dedupe case-insensitive, sin pisar el PATH system); `-Uninstall`; parámetros
  `-InstallDir` y `-NoPath` (para pruebas y uso avanzado) <!-- id: 1 -->
- [ ] `docs/install.md`: prerequisitos, comandos por SO, qué hace (compilación
  local, dónde cae, PATH de usuario), limitaciones (PATH en terminales NUEVAS;
  no instalar en system), desinstalar <!-- id: 2 -->
- [ ] Verificación funcional: correr `install.sh` con `HOME` temporal (crea
  binario y bloque en los rcs) y `install.ps1` con `-InstallDir` temporal y
  `-NoPath` (crea el exe, no toca el PATH real); `sh -n`/sintaxis <!-- id: 3 -->
- [ ] Commit de unidad <!-- id: 4 -->

## Design decisions
- **Compilación local, no binarios distribuidos:** no hay releases/CI; el
  instalador requiere Go y genera el ejecutable de la plataforma en el
  destino. Documentado como requisito.
- **`~/.tcode` ya existe como home de config:** `bin/` va dentro, sin tocar
  `config.json`/`theme.json`/`crash.log`.
- **PATH de USUARIO, nunca system:** Windows vía registro 'User' (setx
  trunca a 1024 y puede pisar el PATH con variables); Unix vía los rc del
  shell con bloque marcado para idempotencia.
- **Pruebas en sandbox:** `HOME`/`-InstallDir` temporales + `-NoPath`;
  jamás se toca el PATH o el `~/.tcode` real del host durante la verificación.