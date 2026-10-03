# Feature: Detección de Cambios Externos

## Description
Guardar pisaba el archivo sin avisar si otro proceso lo había modificado mientras
tanto. Ahora el guardado lo detecta y se niega, con una vía explícita para forzar.

Commit: `be9b6fe`.

## Tasks
- [x] Registrar tamaño y fecha de modificación al cargar y al guardar <!-- id: 0 -->
- [x] `ChangedOnDisk` comparando esa marca <!-- id: 1 -->
- [x] `Save` devuelve `ErrFileChangedExternally` en lugar de pisar <!-- id: 2 -->
- [x] `SaveForce` para forzar, con la decisión explícita <!-- id: 3 -->
- [x] `Ctrl+S` avisa; el segundo `Ctrl+S` pisa <!-- id: 4 -->
- [x] Cualquier otra tecla retira el permiso de pisar <!-- id: 5 -->
- [x] Tests, incluido el peligro del `mmap` vivo <!-- id: 6 -->
- [x] Verificación: `go vet`, `gofmt -l`, `go test -race` <!-- id: 7 -->
- [x] Commit de unidad de trabajo <!-- id: 8 -->

## Design decisions

### Detección por metadatos, con sus límites declarados
`ChangedOnDisk` compara tamaño y fecha de modificación contra la marca registrada al
cargar o guardar. Si el archivo **no se puede consultar** —borrado, permisos— cuenta
como cambiado: avisar de más es más seguro que pisar en silencio.

Es una detección por metadatos, no una garantía de integridad: un cambio que deje el
mismo tamaño y la misma fecha pasa desapercibido. Se documenta como tal en el código
en lugar de presentarla como una verificación fuerte.

### Sin cambios locales, no hay nada que proteger
`Save` refresca la marca del disco cuando no hay cambios que escribir: no puede
perderse nada, así que seguir avisando sería una falsa alarma permanente.

### El permiso de pisar se retira con cualquier otra tecla
Igual que la confirmación de salida. Un `Ctrl+S` lejano no debe pisar sin avisar
porque alguien aceptó una advertencia hace rato.

## Evidence

### El hallazgo: el `mmap` es una **vista viva**
El test que afirmaba que el documento en memoria quedaba intacto tras una escritura
externa **falló**, y el contenido observado fue `"propio esc"` en lugar de
`"propio uno"`.

`os.WriteFile` trunca y reescribe **el mismo inodo** que tenemos mapeado. Como las
piezas del documento apuntan a ese mapeo, **el contenido del editor cambió por
debajo**: estaba leyendo el texto que escribió el otro proceso, mezclado con el
propio.

Esto es más grave que pisar a otro, y reencuadra la unidad entera: la detección **no
es una comodidad para no pisar, es la única señal de que el documento en memoria
dejó de ser confiable.**

Y hay un filo peor: si la escritura en el lugar deja el archivo **más corto** que el
mapeo, leer las páginas que quedaron fuera del archivo levanta **SIGBUS** y mata el
proceso. Es el mismo mecanismo ya registrado en `odd/tasks/save-to-disk.md` para el
guardado en el lugar.

Los tests ahora separan los dos fenómenos:

- `writeExternally` usa **temporal y renombre**, como hacen los editores. El inodo
  cambia, nuestro `mmap` sigue viendo lo viejo, y el documento no se altera.
- `TestInPlaceExternalWriteChangesTheMappedContent` usa escritura **en el lugar** y
  fija el comportamiento real: tras el cambio, `GetContent()` devuelve `"EXT"` —los
  primeros bytes del mapeo, que ahora reflejan el archivo reescrito— y no `"uno"`.

### Verificación
```
go vet ./...              → limpio
gofmt -l .                → limpio
go test -race -count=1    → pasa (el total subió a 176 tras esta unidad)
```

### Documentación de la convención de tests
En esta misma unidad se agregó la sección **4.1 Layout de los tests** a
`docs/constitution.md`, explicando por qué los `_test.go` van junto al código.

## Known Limitations
- **Sin recarga.** Detectar el cambio no alcanza: el documento en memoria ya puede
  estar contaminado por una escritura en el lugar. Recargar el archivo al detectar el
  cambio es el paso natural siguiente, y es seguro cuando no hay cambios locales.
- **Sin fusión.** Forzar el guardado descarta el trabajo del otro proceso sin
  ofrecer combinarlo.
- **Ventana de carrera.** Entre la verificación y el renombre, otro proceso puede
  escribir.
- La detección no corre sola: se consulta al guardar y en los tests. No hay
  vigilancia del archivo en segundo plano.
