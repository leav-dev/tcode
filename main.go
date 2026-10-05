package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tcode/internal/controller"
	"tcode/internal/ext"
)

// Modos comando de la CLI: instalan, listan o eliminan extensiones sobre la
// raíz de USUARIO (~/.tcode/extensions) y salen sin abrir la UI. El
// instalador escribe solo ahí: las raíces de proyecto y usuario las resuelve
// el controller al arrancar, y esta CLI no toca esa lógica. Los flags de
// proveedores manejan ~/.tcode/providers.json, la lista de fuentes desde la
// que se resuelve cada id de extensión.
const (
	flagInstallExtension = "--install-extension"
	flagListExtensions   = "--list-extensions"
	flagRemoveExtension  = "--remove-extension"
	flagAddProvider      = "--add-provider"
	flagApproveProvider  = "--approve-provider"
)

func main() {
	if code, ok := runCommandMode(); ok {
		os.Exit(code)
	}

	updateExtensionsAtStartup()

	path := ""
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	app, err := controller.NewApp(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tcode: %v\n", err)
		os.Exit(1)
	}
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tcode: %v\n", err)
		os.Exit(1)
	}
}

// updateExtensionsAtStartup revisa las extensiones instaladas y actualiza las
// que el proveedor ya no tiene en la versión instalada. Corre ANTES de abrir la
// UI: así el editor arranca con la versión nueva ya en disco, sin recargar
// extensiones a mitad del arranque.
//
// Nunca impide arrancar: si no se puede resolver el home, leer los
// proveedores o volver a leer uno remoto, el problema se reporta por stderr y
// el editor sigue con lo que ya está instalado.
func updateExtensionsAtStartup() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tcode: aviso: no se pudo resolver el home del usuario para actualizar extensiones: %v\n", err)
		return
	}
	providers, err := ext.AllProviders(ext.ProvidersFilePath(home))
	if err != nil {
		fmt.Fprintf(os.Stderr, "tcode: aviso: %v\n", err)
		return
	}
	updates, errs := ext.UpdateAll(providers, filepath.Join(home, ".tcode", "extensions"), nil)
	for _, u := range updates {
		fmt.Printf("Actualizada: %s (%s → %s)\n", u.Ref, u.OldVer, u.NewVer)
	}
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "tcode: aviso: %v\n", e)
	}
}

// runCommandMode ejecuta el modo comando cuando os.Args[1] es un flag de
// extensión. Devuelve (código de salida, cierto) cuando hay que salir sin
// abrir la UI; con cualquier otro primer argumento (o ninguno) devuelve
// (0, falso) y main arranca la app normal.
func runCommandMode() (int, bool) {
	args := os.Args[1:]
	if len(args) == 0 {
		return 0, false
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tcode: resolviendo el home del usuario: %v\n", err)
		return 1, true
	}
	userRoot := filepath.Join(home, ".tcode", "extensions")
	providersFile := ext.ProvidersFilePath(home)

	switch args[0] {
	case flagInstallExtension:
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "tcode: uso: tcode --install-extension <id>")
			return 1, true
		}
		// Instalar por URL era el flujo anterior: una URL es hoy una FUENTE, no
		// una extensión. Decirlo evita el error críptico de "no se encontró la
		// extensión https://…".
		if ext.IsRemoteSource(args[1]) {
			fmt.Fprintf(os.Stderr, "tcode: %q es una fuente, no una extensión\n", args[1])
			fmt.Fprintf(os.Stderr, "tcode: registrala con %s %s y después instalá por id\n", flagAddProvider, args[1])
			return 1, true
		}
		providers, err := ext.AllProviders(providersFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tcode: %v\n", err)
			return 1, true
		}
		res, err := ext.InstallByID(args[1], providers, userRoot, nil, confirmProvider)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tcode: %v\n", err)
			return 1, true
		}
		fmt.Printf("Instalada: %s\n", res.Ref())
		fmt.Println("disponible en la próxima sesión")
		return 0, true

	case flagRemoveExtension:
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "tcode: uso: tcode --remove-extension <proveedor:id| id>")
			return 1, true
		}
		ref, err := ext.RemoveRef(userRoot, args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "tcode: %v\n", err)
			return 1, true
		}
		fmt.Printf("Eliminada: %s\n", ref)
		return 0, true

	case flagListExtensions:
		infos, errs := ext.List(userRoot)
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "tcode: aviso: %v\n", e)
		}
		if len(infos) == 0 {
			fmt.Println("sin extensiones instaladas")
		}
		for _, i := range infos {
			fmt.Printf("%s (%s) v%s\n", i.Ref(), i.Name, i.Version)
		}
		return 0, true

	case flagAddProvider:
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "tcode: uso: tcode --add-provider <url-git|carpeta>")
			return 1, true
		}
		if err := addProvider(args[1], providersFile); err != nil {
			fmt.Fprintf(os.Stderr, "tcode: %v\n", err)
			return 1, true
		}
		return 0, true

	case flagApproveProvider:
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "tcode: uso: tcode --approve-provider <nombre>")
			return 1, true
		}
		if err := approveProvider(args[1], providersFile); err != nil {
			fmt.Fprintf(os.Stderr, "tcode: %v\n", err)
			return 1, true
		}
		return 0, true
	}

	return 0, false
}

// addProvider registra una fuente de extensiones en ~/.tcode/providers.json:
// valida que tenga la forma de proveedor (subcarpetas con extension.json),
// deriva su nombre y la guarda SIN aprobar. La aprobación es una decisión
// posterior y explícita del usuario (--approve-provider), porque agrega una
// fuente no significa confiar en ella.
func addProvider(source, providersFile string) error {
	// Una fuente que no es ni una URL ni una carpeta existente es un error de
	// tipeo: sin este chequeo, derivaría un nombre y recién al leer el
	// proveedor fallaría con un mensaje de git que no lo dice.
	if !ext.IsLocalSource(source) && !ext.IsRemoteSource(source) {
		return fmt.Errorf("%q no es una URL de git ni una carpeta existente", source)
	}
	name, err := ext.DeriveName(source)
	if err != nil {
		return err
	}
	canonical, err := ext.CanonicalSource(source)
	if err != nil {
		return err
	}
	p := ext.Provider{Name: name, Source: canonical, Approved: false}

	// El nombre es la identidad del namespacing: un duplicado haría que dos
	// fuentes escribieran bajo la misma carpeta y que la segunda quedara
	// inalcanzable por id.
	current, err := ext.AllProviders(providersFile)
	if err != nil {
		return err
	}
	for _, other := range current {
		if other.Name != name {
			continue
		}
		if other.Source == canonical {
			return fmt.Errorf("la fuente %q ya está registrada como el proveedor %q", source, name)
		}
		return fmt.Errorf("ya existe un proveedor llamado %q, que apunta a otra fuente", name)
	}

	exts, err := ext.ValidateProviderSource(p, nil)
	if err != nil {
		return err
	}
	stored, err := ext.LoadProviders(providersFile)
	if err != nil {
		return err
	}
	if err := ext.SaveProviders(providersFile, append(stored, p)); err != nil {
		return err
	}

	fmt.Printf("Proveedor agregado: %s\n", name)
	fmt.Printf("fuente: %s\n", canonical)
	fmt.Printf("extensiones disponibles: %d\n", len(exts))
	fmt.Println("todavía no está aprobado: al instalar desde ahí te vamos a pedir confirmación")
	return nil
}

// approveProvider marca un proveedor guardado como aprobado: a partir de ahí
// instalar desde él no pregunta. El proveedor por defecto ya está aprobado y no
// vive en el config, así que aprobarlo es un no-op que se informa, no un error.
func approveProvider(name, providersFile string) error {
	stored, err := ext.LoadProviders(providersFile)
	if err != nil {
		return err
	}
	for i, p := range stored {
		if p.Name != name {
			continue
		}
		if p.Approved {
			fmt.Printf("El proveedor %q ya estaba aprobado\n", name)
			return nil
		}
		stored[i].Approved = true
		if err := ext.SaveProviders(providersFile, stored); err != nil {
			return err
		}
		fmt.Printf("Proveedor aprobado: %s\n", name)
		return nil
	}
	if name == ext.DefaultProvider().Name {
		fmt.Printf("El proveedor por defecto %q ya está aprobado\n", name)
		return nil
	}
	return fmt.Errorf("no hay ningún proveedor llamado %q", name)
}

// confirmProvider pregunta al usuario si confía en un proveedor sin aprobar.
// El default es NO: cualquier cosa que no sea un sí explícito (vacío, "n", EOF
// sin TTY) cancela la instalación. El reader se crea una sola vez para no
// perder la primera línea leída entre varias preguntas.
var stdinReader = bufio.NewReader(os.Stdin)

func confirmProvider(p ext.Provider) bool {
	fmt.Printf("El proveedor %q no está aprobado.\n", p.Name)
	fmt.Printf("fuente: %s\n", p.Source)
	fmt.Print("¿Confías en este proveedor? [s/N]: ")

	line, err := stdinReader.ReadString('\n')
	if err != nil && line == "" {
		fmt.Println()
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "s", "si", "sí", "y", "yes":
		return true
	}
	return false
}
