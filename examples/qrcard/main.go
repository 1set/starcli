// qrcard shows how a build-your-own shell wires a specific starpkg module: it
// imports just github.com/starpkg/qrcode and hands its loader to the kit. The
// turnkey starcli wires every starpkg module; this shell wires exactly one.
package main

import (
	_ "embed"
	"log"

	"github.com/1set/starcli/kit"
	"github.com/starpkg/qrcode"
)

//go:embed app.star
var app string

func main() {
	if _, err := kit.Run(app, kit.WithLoader(qrcode.ModuleName, qrcode.NewModule().LoadModule())); err != nil {
		log.Fatal(err)
	}
}
