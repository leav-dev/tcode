package ext

import (
	"fmt"
)

// Snapshot es la foto completa del estado de extensiones en un instante: los
// proveedores registrados, el catálogo que CADA uno de ellos ofrece y lo que
// está instalado localmente.
//
// Existe para no repetir la parte cara: leer el catálogo de un proveedor es un
// partial clone contra GitHub (medido en este repo: ~3,8 s). Antes, detectar
// actualizaciones y listar novedades llamaban a ListExtensions por separado y
// el arranque pagaba la lectura dos veces (~7 s). Con el snapshot se lee UNA
// vez y las dos vistas se DERIVAN de esa misma lectura.
//
// Las actualizaciones y las novedades no son campos del snapshot porque no son
// datos nuevos: son una comparación entre el catálogo y lo instalado, y por eso
// son métodos (Updates/Available). Tras instalar, actualizar o borrar solo
// cambia la lista local: alcanza con releer List y volver a derivar, sin volver
// a tocar la red.
type Snapshot struct {
	// Providers son los proveedores registrados, en orden de resolución: ante
	// el mismo id en dos proveedores gana el primero.
	Providers []Provider
	// Catalogs mapea el NOMBRE del proveedor a lo que ofrece. Solo contiene los
	// proveedores que se pudieron leer: uno caído simplemente no está, y los
	// métodos derivados lo saltan (LoadAll ya reportó el error de lectura).
	Catalogs map[string][]ProviderExt
	// Installed es lo instalado en la raíz de usuario, tal cual lo devuelve
	// List (incluye las instalaciones planas heredadas, sin proveedor). Es el
	// único campo que cambia tras una acción, y basta con releer List.
	Installed []Info
}

// LoadAll lee el catálogo de cada proveedor UNA vez y devuelve el Snapshot con
// lo instalado. Es tolerante como toda la maquinaria de proveedores: un repo
// caído, un nombre de proveedor inválido o un problema al listar lo instalado se
// acumulan en errs y NO cortan la lectura de los demás.
//
// Los errores de lectura de un proveedor no se repiten en Updates/Available:
// se reportan acá, una sola vez, y los métodos derivados trabajan sobre lo que
// sí se pudo leer.
func LoadAll(providers []Provider, userRoot string, fetcher FetchFunc) (Snapshot, []error) {
	if fetcher == nil {
		fetcher = fetch
	}

	infos, errs := List(userRoot)
	snap := Snapshot{
		Providers: providers,
		Catalogs:  make(map[string][]ProviderExt, len(providers)),
		Installed: infos,
	}
	for _, p := range providers {
		// Un nombre fuera de la gramática sería una carpeta de instalación
		// insegura: ni se lee ni se ofrece.
		if !providerNameRe.MatchString(p.Name) {
			errs = append(errs, fmt.Errorf("proveedor con nombre inválido %q: se ignora", p.Name))
			continue
		}
		exts, err := ListExtensions(p, fetcher)
		if err != nil {
			// Proveedor caído: no impide leer los siguientes.
			errs = append(errs, fmt.Errorf("revisando el proveedor %q: %w", p.Name, err))
			continue
		}
		snap.Catalogs[p.Name] = exts
	}
	return snap, errs
}

// Updates deriva las actualizaciones disponibles SIN releer nada: compara el
// catálogo cacheado contra lo instalado. Es el mismo criterio de CheckUpdates
// (la copia instalada no tiene .git al que hacele fetch, así que la detección
// es comparar versiones) con la misma tolerancia: una instalación heredada sin
// proveedor, un proveedor que ya no ofrece la extensión instalada o uno que ya
// no está registrado se reportan como error y no cortan la revisión de las
// demás. Los proveedores cuyo catálogo no se pudo leer se saltan en silencio:
// LoadAll ya lo reportó.
//
// Los ids con el mismo nombre en dos proveedores se resuelven por el primero,
// igual que al instalar: el catálogo de cada uno solo se compara contra lo que
// salió de él.
func (s Snapshot) Updates() ([]UpdateResult, []error) {
	byProvider := map[string][]Info{}
	var errs []error
	for _, info := range s.Installed {
		if info.Provider == "" {
			// Instalación plana heredada: no hay contra qué comparar.
			errs = append(errs, fmt.Errorf("la extensión %q no tiene proveedor: no se puede actualizar", info.ID))
			continue
		}
		byProvider[info.Provider] = append(byProvider[info.Provider], info)
	}

	var updates []UpdateResult
	seen := map[string]bool{}
	for _, p := range s.Providers {
		inst := byProvider[p.Name]
		if len(inst) == 0 {
			continue
		}
		seen[p.Name] = true
		catalog, ok := s.Catalogs[p.Name]
		if !ok {
			continue // proveedor ilegible: el error ya lo dio LoadAll
		}
		for _, info := range inst {
			pe, ok := findProviderExt(catalog, info.ID)
			if !ok {
				errs = append(errs, fmt.Errorf("el proveedor %q ya no ofrece la extensión %q instalada", p.Name, info.ID))
				continue
			}
			if pe.Version == info.Version {
				continue
			}
			updates = append(updates, UpdateResult{Ref: info.Ref(), OldVer: info.Version, NewVer: pe.Version})
		}
	}

	// Instaladas cuyo proveedor no está en la lista vigente: sin fuente contra
	// la cual comparar. Se recorren en orden alfabético para que el reporte sea
	// estable entre corridas.
	for _, name := range sortedKeys(byProvider) {
		if seen[name] {
			continue
		}
		for _, info := range byProvider[name] {
			errs = append(errs, fmt.Errorf("la extensión %s viene del proveedor %q, que ya no está registrado", info.Ref(), name))
		}
	}
	return updates, errs
}

// Available deriva las novedades sin releer nada: el catálogo cacheado menos lo
// instalado, por proveedor. Es el mismo criterio de AvailableExtensions —una
// extensión instalada desde otro proveedor no tapa la novedad del que la
// ofrece, y ante el mismo id en dos proveedores gana el primero—. No devuelve
// errores porque no tiene nada que tolerar: todo lo que puede estar mal (un
// nombre de proveedor inválido, un repo caído) ya lo reportó LoadAll, y lo que
// no se pudo leer no se ofrece.
func (s Snapshot) Available() []AvailableExt {
	// Info.Ref() da "proveedor/id" para lo namespaced y el id suelto para las
	// instalaciones planas heredadas, que así no tapan nada por proveedor.
	installed := make(map[string]bool, len(s.Installed))
	for _, info := range s.Installed {
		installed[info.Ref()] = true
	}

	var available []AvailableExt
	seen := map[string]bool{}
	for _, p := range s.Providers {
		catalog, ok := s.Catalogs[p.Name]
		if !ok {
			continue
		}
		for _, pe := range catalog {
			ref := p.Name + "/" + pe.ID
			if seen[pe.ID] || installed[ref] {
				continue
			}
			seen[pe.ID] = true
			available = append(available, AvailableExt{
				Provider: p,
				ID:       pe.ID,
				Name:     pe.Name,
				Version:  pe.Version,
				Subdir:   pe.Subdir,
			})
		}
	}
	return available
}
