# Feature: Optimización de los Índices de la Piece Table

## Description
Los benchmarks mostraron que el **render ya estaba resuelto** pero la **edición no**:
cada tecla costaba cientos de microsegundos en un archivo grande. La causa era que
el índice de líneas se **alocaba y se copiaba entero en cada edición**.

Commit: la unidad de optimización.

## Tasks
- [x] Benchmarks que midan el costo real antes de tocar nada <!-- id: 0 -->
- [x] Corrimiento del índice de líneas **en el lugar**, sin alocación <!-- id: 1 -->
- [x] Compactación en el lugar en el borrado <!-- id: 2 -->
- [x] Verificar que los 192 tests sigan pasando <!-- id: 3 -->
- [x] Volver a medir para confirmar la mejora <!-- id: 4 -->
- [x] Verificación: `go vet`, `gofmt -l`, `go test -race` <!-- id: 5 -->
- [x] Commit de unidad de trabajo <!-- id: 6 -->

## Design decisions

### Medir primero
La unidad arrancó escribiendo `bench_test.go` y corriendo los benchmarks **antes** de
cambiar una línea. Sin eso no se sabe qué optimizar, y es fácil "optimizar" lo que ya
era rápido.

La medición dejó el diagnóstico servido:

| Operación (100k líneas) | Antes |
|---|---|
| `GetRange` ventana visible de 50 líneas | **11,9 ns** (1k) / **18,5 ns** (100k) |
| Tipear al final | 183 µs |
| Tipear al principio | 245 µs |
| Tipear en el medio | 273 µs |
| Tipear un salto al final | 231 µs |
| Backspace | 499 µs |
| `locate` con 400 piezas | 174 ns |

**El render es gratis y no depende del tamaño del archivo.** La premisa del proyecto
—"lo que no está en pantalla no se carga"— se sostiene: pedir la ventana visible
cuesta nanosegundos y no crece con el archivo. El problema estaba **exclusivamente en
la edición**.

### La causa: una copia completa del índice por tecla
`insertIntoLineOffsets` armaba un slice nuevo con `make` y copiaba **todas** las
líneas, incluso al escribir al final, donde no hay ninguna que correr.
`deleteFromLineOffsets` hacía lo mismo. Era una alocación más una copia completa del
array de líneas por cada carácter tipeado.

### La corrección: trabajar en el mismo slice
- **Inserción sin saltos** (el caso normal al tipear): solo se corre la cola
  `+length` **en el mismo slice**. Escribir al final no toca ni una entrada. Cero
  alocaciones.
- **Inserción con saltos:** se crece, se corre la cola con `copy` —que resuelve el
  solapamiento— y se escriben los nuevos en el hueco.
- **Borrado:** se compacta en el lugar. La escritura va siempre a una posición
  **menor o igual** a la que se lee, así que no hace falta un buffer aparte.

## Evidence

### Resultado de la medición, antes y después

| Operación (100k líneas) | Antes | Después | Mejora |
|---|---|---|---|
| Tipear al final | 183 µs | **~0,8 µs** | ~230× |
| Tipear un salto al final | 231 µs | **~0,8 µs** | ~285× |
| **Backspace** | 499 µs | **~0,18 µs** | ~2800× |
| Tipear en el medio | 273 µs | ~27 µs | 10× |
| Tipear al principio | 245 µs | ~49 µs | 5× |
| `GetRange` (render) | 12–19 ns | 14–19 ns | sin cambio |

El borrado mejora más que la inserción porque antes hacía **dos** copias completas
del índice —una por el borrado y otra por la reinserción— y ahora son dos
corrimientos sin alocación.

Números tomados con `-benchtime=3000x -count=2`; las dos corridas coinciden. Los
casos por debajo del microsegundo varían algo entre corridas por efectos de
alocación y caché, así que se reportan como órdenes de magnitud, no como cifras
exactas.

### Corrección antes que velocidad
Los 192 tests, incluidos los diferenciales de 400 y 120 ediciones con verificación de
invariantes, se corrieron **antes** de aceptar la mejora. Una optimización que rompe
el índice de líneas no es una mejora.

### Verificación
```
go test ./internal/model/ -run '^$' -bench . -benchtime=3000x   → ver tabla
go vet ./...              → limpio
gofmt -l .                → limpio
go test -race -count=1    → 192 tests OK
```

## Known Limitations

**Esta unidad NO elimina la complejidad asintótica.** Saca la alocación y la copia
completa, que era el costo dominante, pero queda lo inherente a un array:

- **Tipear en el medio o al principio sigue siendo O(líneas).** La cola *realmente*
  tiene que correrse: con un array plano no hay forma de evitarlo. Los ~49 µs del
  peor caso son eso. **Sacarlo requiere cambiar la estructura**, no la implementación:
  un árbol de orden sobre las longitudes de línea (tipo Fenwick) daría O(log líneas)
  para buscar, insertar y desplazar.
- **`locate` sigue siendo O(piezas).** Con 400 piezas costaba 174 ns, que es barato en
  absoluto, pero crece linealmente. Necesita un árbol balanceado sobre las piezas
  —un treap implícito o un rope— para pasar a O(log piezas).
- **Sin memoria de trabajo acotada:** el índice crece con las líneas editadas.

Ninguna de las dos estructuras es una continuación de esta unidad: son reescrituras
del corazón del modelo, de riesgo alto, y merecen su propia unidad con sus propios
diferenciales. Se declaran acá para no vender esta optimización como más de lo que
es: **es una mejora de constante grande —hasta tres órdenes de magnitud en el caso
más común— sin cambiar el orden de complejidad.**
