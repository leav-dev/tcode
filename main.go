package main

import (
	"fmt"
	"os"
	"path/filepath"

	"tcode/internal/controller"
	"tcode/internal/ext"
)

// Modos comando de la CLI: instalan, listan o eliminan extensiones sobre la
// raíz de USUARIO (~/.tcode/extensions) y salen sin abrir la UI. El
// instalador escribe solo ahí: las raíces de proyecto y usuario las resuelve
// el controller al arrancar, y esta CLI no toca esa lógica.
const (
	flagInstallExtension = "--install-extension"
	flagListExtensions   = "--list-extensions"
	flagRemoveExtension  = "--remove-extension"
)

func main() {
	if code, ok := runCommandMode(); ok {
		os.Exit(code)
	}

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

	switch args[0] {
	case flagInstallExtension:
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "tcode: uso: tcode --install-extension <url>")
			return 1, true
		}
		id, err := ext.InstallFromGit(args[1], userRoot, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tcode: %v\n", err)
			return 1, true
		}
		fmt.Printf("Instalada: %s\n", id)
		fmt.Println("disponible en la próxima sesión")
		return 0, true

	case flagRemoveExtension:
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "tcode: uso: tcode --remove-extension <id>")
			return 1, true
		}
		if err := ext.Remove(userRoot, args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "tcode: %v\n", err)
			return 1, true
		}
		fmt.Printf("Eliminada: %s\n", args[1])
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
			fmt.Printf("%s (%s) v%s\n", i.ID, i.Name, i.Version)
		}
		return 0, true
	}

	return 0, false
}
