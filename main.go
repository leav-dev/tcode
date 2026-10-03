package main

import (
	"fmt"
	"os"

	"tcode/internal/controller"
)

func main() {
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
