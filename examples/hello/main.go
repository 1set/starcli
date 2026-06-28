// hello is the smallest build-your-own shell: embed one .star file and run it
// with a couple of builtin modules. This is the entire program — the business
// logic lives in app.star, the Go side is just the shell.
package main

import (
	_ "embed"
	"log"

	"github.com/1set/starcli/kit"
)

//go:embed app.star
var app string

func main() {
	if _, err := kit.Run(app, kit.WithModules("math")); err != nil {
		log.Fatal(err)
	}
}
