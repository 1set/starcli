# Build your own — starcli `kit` examples

`starcli` is the **standard, fully-loaded** Star\* CLI. But it is built on a small
reusable core — the [`kit`](../kit) package — that you can use to assemble your
**own** CLI: a single Go shell that embeds your Starlark scripts and wires only
the modules you need. The standard `starcli` and your shell construct their
runtime through exactly the same path, so they behave the same.

> **The idea:** Go is the shell, Starlark is the app. Your `main.go` stays a few
> lines and never changes; your logic lives in `.star` files you `//go:embed`.

This folder is a **separate Go module** on purpose: each demo depends on only the
runtime plus the modules it wires, so its `go.mod` is the honest, minimal
dependency tree a real shell would have.

## Run the demos

```bash
cd examples
go run ./hello      # embed one .star, run it with a builtin module
go run ./qrcard     # wire one starpkg module (qrcode) into the shell
```

## `hello` — the smallest shell

The entire program is the embed + one call. Everything else is Starlark.

```go
//go:embed app.star
var app string

func main() {
	if _, err := kit.Run(app, kit.WithModules("math")); err != nil {
		log.Fatal(err)
	}
}
```

## `qrcard` — wiring a starpkg module

A build-your-own shell picks the domain modules it wants and hands their loaders
to the kit. Here the shell imports just `github.com/starpkg/qrcode`:

```go
import "github.com/starpkg/qrcode"

func main() {
	kit.Run(app, kit.WithLoader(qrcode.ModuleName, qrcode.NewModule().LoadModule()))
}
```

Its dependency tree is `starbox` + `qrcode` and nothing else — none of the other
starpkg modules the turnkey `starcli` carries.

## How modules resolve

`kit` follows starbox's resolution order, so you mix three styles freely:

| You write | Resolves as | Use for |
|---|---|---|
| `kit.WithModules("json", "math")` | starlet **builtins** (auto) | the standard library |
| `kit.WithLoader("qrcode", loader)` | an **explicit** loader you import | a specific starpkg/custom module |
| `kit.WithDynamicLoader(fn)` | a **registry** resolved on demand | exposing many modules at once (this is how `starcli` itself wires every starpkg module) |

## Shipping a tree of scripts

For more than one script, embed a whole directory and run an entry point — the
other files are reachable via `load()`:

```go
//go:embed scripts/*.star
var scripts embed.FS

func main() {
	kit.RunFS(scripts, "scripts/main.star", kit.WithModules("json"))
}
```

## Common knobs

```go
kit.Run(app,
	kit.WithModules("json", "math"),
	kit.WithGlobal("env", "prod"),       // inject host values as script globals
	kit.WithMaxSteps(1_000_000),         // bound runaway loops
	kit.WithMaxOutputEntries(100),       // bound result size
	kit.WithPrintFunc(myPrinter),        // control how print() renders
)
```

See the [`kit` package docs](../kit/kit.go) for the full option set.
