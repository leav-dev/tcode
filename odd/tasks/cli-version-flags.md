# Feature: `tcode --version` + guía ante flags desconocidos

## Description
`--version` no existía y cualquier flag con typo caía al editor como si fuera
un archivo a abrir (`tcode --versoin` abría un buffer llamado así).

## Decisiones de arquitectura (del agente)
- `--version` (alias `-v`) imprime `update.CurrentVersion()` (ldflags en
  releases) o marca dev. Helper puro `versionLine()` testeable.
- Un primer argumento con `-` que no es flag conocido falla fuerte (exit 1)
  con la guía completa de comandos + sugerencia por prefijo o Levenshtein
  ≤ 2 (`--versoin` → `--version`). Rutas normales (archivos/carpetas)
  siguen abriendo el editor igual que antes.
- Tests en `main_test.go` (paquete main no tenía): versión, sugerencias,
  cobertura de la guía contra los flags del switch.

## Tasks
- [x] `main.go`: `flagVersion`, `versionLine`, `commandGuideLines`,
  `suggestFlag`+`editDistance`, rama en el switch + detector `HasPrefix -`
- [x] `--help`/`-h`: `printHelp` (uso + glosario) con `printCommandGuide`
  compartido para que la guía nunca diverja; `--help` también en la guía
- [x] `main_test.go`: versión, prefijos/typos, guía cubre comandos
- [x] Docs: README (fila Versión + nota de canal en update)

## Evidence
- `go test .` OK; demo viva: `--versoin` sugiere y lista, exit 1;
  `--version`/`-v` imprimen el tag.
