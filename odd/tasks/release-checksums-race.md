# Fix: checksums.txt con carrera + instalador que moría en silencio

## Description
El release `preview-v0.1.3` se publicó pero `install.sh --preview` moría con
exit 1 sin mensaje tras descargar. Dos bugs encadenados:

1. **Carrera en el workflow**: los 4 jobs de la matriz subían `dist/*` al mismo
   release y cada uno sobrescribía el `checksums.txt` del otro. El archivo final
   tenía UNA sola entrada (al azar) y con prefijo `dist/`. Rompe la verificación
   en el instalador y en `tcode update` (v0.1.2 estable tiene el mismo problema:
   solo Windows verifica bien).
2. **`set -e` en el instalador**: `ASSET="...$( [ .. ] && .. )"` con test falso
   mata el script en silencio (el fallo se hereda en la sustitución). Lo mismo
   para el `grep` del asset en `checksums.txt` cuando no hay coincidencia.

## Decisiones de arquitectura
- Workflow en dos fases: la matriz compila y sube cada binario como artifact
  (`asset-<target>`); un job `publish` los fusiona, genera UN `checksums.txt`
  con nombres pelados y sube todo una sola vez al release.
- Instalador: el sufijo `.exe` va con `if` (no con `$(.. && ..)`); el `grep` sin
  coincidencia falla con error claro en vez de muerte silenciosa.

## Tasks
- [x] Reparar el asset publicado: `checksums.txt` correcto subido con
  `gh release upload --clobber` a `preview-v0.1.3`
- [x] `.github/workflows/release.yml`: jobs build (artifacts) + publish (merge)
- [x] `scripts/install.sh`: fix `set -e` + grep con error fuerte
- [x] Evidencia: `bash -n`, file:// feliz, error fuerte, instalación real OK

## Evidence
- `preview-v0.1.3/checksums.txt` publicado tenía 90 bytes / 1 línea
  (`dist/tcode-darwin-arm64`); `v0.1.2` igual (solo Windows).
- `bash -x` muere tras `ASSET=...`; replicado mínimo confirma el `set -e`.
- Instalación real desde el CDN verifica (`sha256sum -c` OK) e instala.
- PENDIENTE (no pedido): reparar `checksums.txt` de `v0.1.2` estable con el
  mismo procedimiento (`gh release upload --clobber`).
