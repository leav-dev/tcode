package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leav-dev/tcode/internal/controller"
	"github.com/leav-dev/tcode/internal/ext"
	"github.com/leav-dev/tcode/internal/update"
)

// Modos comando de la CLI: instalan, listan o eliminan extensiones sobre la
// raíz de USUARIO (~/.tcode/extensions) y salen sin abrir la UI. El
// instalador escribe solo ahí: las raíces de proyecto y usuario las resuelve
// el controller al arrancar, y esta CLI no toca esa lógica. Los flags de
// proveedores manejan ~/.tcode/providers.json, la lista de fuentes desde la
// que se resuelve cada id de extensión.
const (
	flagHelp             = "--help"
	flagVersion          = "--version"
	flagInstallExtension = "--install-extension"
	flagListExtensions   = "--list-extensions"
	flagRemoveExtension  = "--remove-extension"
	flagAddProvider      = "--add-provider"
	flagApproveProvider  = "--approve-provider"
)

// commandGuideLines es la guía de comandos que se muestra ante un flag
// desconocido: qué se puede pedir sin abrir el editor.
var commandGuideLines = []string{
	"--help · muestra esta ayuda",
	"--version · muestra la versión del editor",
	"--install-extension <id> · instala una extensión por id",
	"--list-extensions · lista las instaladas",
	"--remove-extension <proveedor:id|id> · borra una extensión",
	"--add-provider <url-git|carpeta> · registra una fuente",
	"--approve-provider <nombre> · confía en una fuente",
	"update · actualiza el editor a lo último de su canal",
	"uninstall · desinstala el editor (binario + PATH, conserva config y extensiones)",
}

// versionLine arma la línea de versión sin imprimirla (testeable): la
// inyectada por ldflags en releases, o marca de dev sin ella.
func versionLine() string {
	if v := update.CurrentVersion(); v != "" {
		return "tcode " + v
	}
	return "tcode dev (build de desarrollo)"
}

// guideCommands extrae el comando de cada línea de la guía (su primer
// campo): es lo que suggestFlag compara, parseado en un solo lugar en vez
// de rebanar las líneas en cada pasada.
func guideCommands() []string {
	cmds := make([]string, 0, len(commandGuideLines))
	for _, line := range commandGuideLines {
		cmds = append(cmds, strings.Fields(line)[0])
	}
	return cmds
}

// suggestFlag propone el comando más parecido al flag desconocido: primero
// por prefijo (en ambas direcciones); si no hay, por distancia de edición
// ≤ 2 (typos como --versoin). "" si no hay nada cercano.
func suggestFlag(unknown string) string {
	u := strings.TrimLeft(unknown, "-")
	if u == "" {
		return ""
	}
	cmds := guideCommands()
	for _, cmd := range cmds {
		bare := strings.TrimLeft(cmd, "-")
		if strings.HasPrefix(bare, u) || strings.HasPrefix(u, bare) {
			return cmd
		}
	}
	best, bestDist := "", 3
	for _, cmd := range cmds {
		if d := editDistance(u, strings.TrimLeft(cmd, "-")); d < bestDist {
			best, bestDist = cmd, d
		}
	}
	if bestDist > 2 {
		return ""
	}
	return best
}

// editDistance es Levenshtein básico sobre runas: los comandos son cortos y
// la guía es chica, así que no necesita optimización.
func editDistance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		cur := make([]int, len(br)+1)
		cur[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 0
			if ar[i-1] != br[j-1] {
				cost = 1
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(br)]
}

// printCommandGuide imprime el glosario de comandos en w (stdout para
// --help, stderr para la guía de error). Es la misma lista en ambos lados
// para que nunca diverjan.
func printCommandGuide(w *os.File) {
	fmt.Fprintln(w, "comandos permitidos:")
	for _, line := range commandGuideLines {
		fmt.Fprintf(w, "  %s\n", line)
	}
}

// printHelp muestra uso + glosario (--help / -h) y sale sin abrir el editor.
func printHelp() {
	fmt.Println("tcode — editor de texto en la terminal")
	fmt.Println("uso: tcode [archivo|carpeta] [flag] [...]")
	printCommandGuide(os.Stdout)
}

// printUnknownFlagGuide reporta un flag que no existe con la guía de lo
// permitido (y una sugerencia si hay algo cercano). Un flag con typo antes
// caía al editor como si fuera un archivo a abrir; ahora falla fuerte.
func printUnknownFlagGuide(flag string) {
	fmt.Fprintf(os.Stderr, "tcode: flag desconocido: %q\n", flag)
	if s := suggestFlag(flag); s != "" {
		fmt.Fprintf(os.Stderr, "tcode: ¿quisiste decir %s?\n", s)
	}
	fmt.Fprintln(os.Stderr, "tcode:")
	printCommandGuide(os.Stderr)
}

func main() {
	if code, ok := runCommandMode(); ok {
		os.Exit(code)
	}

	// El chequeo de actualizaciones y novedades NO vive acá: corre dentro del
	// controller, en una goroutine que arranca con la app, y avisa por la barra de
	// estado. Antes se imprimía por stdout, antes de abrir la TUI, así que era
	// invisible; después pasó a bloquear el arranque mientras leía los proveedores
	// (git, segundos), y ahora esa lectura corre en segundo plano.

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

// runCommandMode ejecuta el modo comando cuando os.Args[1] es un flag de
// extensión o el subcomando update. Devuelve (código de salida, cierto) cuando hay que salir sin
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

	case "update":
		if err := runUpdate(); err != nil {
			fmt.Fprintf(os.Stderr, "tcode: %v\n", err)
			return 1, true
		}
		return 0, true

	case flagHelp, "-h":
		printHelp()
		return 0, true

	case flagVersion, "-v":
		fmt.Println(versionLine())
		return 0, true

	case "uninstall":
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "tcode: resolviendo el ejecutable: %v\n", err)
			return 1, true
		}
		removed, err := update.Uninstall(home, exe)
		if len(removed) > 0 {
			fmt.Println("Eliminado:")
			for _, r := range removed {
				fmt.Printf("  %s\n", r)
			}
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "tcode: desinstalación parcial: %v\n", err)
			return 1, true
		}
		fmt.Println("abrí una terminal nueva para salir del PATH de esta sesión")
		return 0, true
	}

	if strings.HasPrefix(args[0], "-") {
		printUnknownFlagGuide(args[0])
		return 1, true
	}

	return 0, false
}

// runUpdate actualiza el editor al último release DE SU CANAL: pregunta el
// tag, compara con la versión propia y, si hay algo nuevo, descarga el asset
// verificado y reemplaza el ejecutable en uso. Un build preview solo mira
// previews (jamás se degrada a estable); el resto mira releases/latest como
// siempre. Ya estar al día no es error: se informa y listo. Sin versión
// propia conocida igual funciona: instala latest a ciegas y avisa que no
// pudo comparar.
func runUpdate() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	current := update.CurrentVersion()
	check := update.CheckLatest
	if update.IsPreviewVersion(current) {
		check = update.CheckLatestPreview
	}
	tag, err := check(ctx)
	if err != nil {
		return err
	}
	if tag == "" {
		if update.IsPreviewVersion(current) {
			return fmt.Errorf("no se pudo saber el último preview (sin red, rate limit o aún no hay previews)")
		}
		return fmt.Errorf("no se pudo saber el último release (sin red o rate limit)")
	}
	if current != "" && !update.NeedsUpdate(current, tag) {
		fmt.Printf("Ya estás al día: %s\n", current)
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolviendo el ejecutable: %w", err)
	}
	if current == "" {
		fmt.Printf("Versión propia desconocida (build de desarrollo); instalando %s en %s\n", tag, exe)
	} else {
		fmt.Printf("Actualizando %s → %s en %s\n", current, tag, exe)
	}
	if err := update.UpdateTo(ctx, tag, exe, update.DownloadBaseFor(tag)); err != nil {
		return err
	}
	fmt.Printf("Actualizado a %s\n", tag)
	return nil
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
