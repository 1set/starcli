# 🛰️ StarCLI

A command-line interface for executing [Starlark](https://github.com/google/starlark-go) scripts with rich module support, interactive mode, and web server capabilities.

## About

StarCLI is a versatile tool that provides a convenient environment for running Starlark scripts from the command line. Starlark is a dialect of Python designed for configuration, extensibility, and embedding. StarCLI extends Starlark with additional modules and utilities to make it more powerful for various automation and scripting tasks.

## Features

- **Multiple Execution Modes**:
  - REPL (Read-Eval-Print Loop) for interactive script development
  - Direct code execution from command line arguments
  - Script file execution
  - Web server mode that creates a Starbox environment for HTTP requests

- **Configurable Environment**:
  - Load custom modules
  - Control recursion and global variable reassignment
  - Set include paths for module loading
  - Configure logging levels

- **Rich Module Support**:
  - Standard library modules (json, math, time, etc.)
  - File system operations
  - Network and HTTP requests
  - Regular expressions
  - Base64 encoding/decoding
  - Email functionality
  - LLM (Language Model) integration
  - Templating, data formats & utilities (Liquid, TOML/YAML, QR codes, TOTP, emoji, in-memory cache)
  - And many more

## Installation

### From Source

Clone the repository and build from source:

```bash
git clone https://github.com/1set/starcli.git
cd starcli
make build
```

### Docker

The project includes a Dockerfile to build and run StarCLI in a container:

```bash
# Build the Docker image
docker build -t starcli .

# Run in interactive mode
docker run -it starcli

# Run a specific script
docker run -v $(pwd):/scripts starcli sh -c "/root/starcli /scripts/your-script.star"
```

## Usage

```bash
$ ./starcli -h
Usage of ./starcli:
      --allow-cmd               enable the cmd module to run ANY host command (no allowlist); also widens a restrictive tier
      --allow-fs                widen a restrictive tier with filesystem modules (file, path)
      --allow-net               widen a restrictive tier with network modules (http, net, email, llm)
      --caps string             capability tier: open (default, everything) | full | network | safe; or env STAR_CAPS
      --check                   syntax/resolve check the script (-c or file) without running it
  -c, --code string             Starlark code to execute
  -C, --config string           config file to load
      --dangerously-allow-all   DANGER: open everything — network + filesystem + host command execution of ANY command. Use only with fully trusted scripts.
  -g, --globalreassign          allow reassigning global variables in Starlark code (default true)
  -I, --include string          include path for Starlark code to load modules from (default ".")
  -i, --interactive             enter interactive mode after executing
  -l, --log string              log level: debug, info, warn, error, dpanic, panic, fatal (default "info")
      --log-file string         append the script's log module output to this file
      --log-format string       log file format: console (human) or json (machine) (default "console")
      --max-output uint         max top-level output entries per run (0=unlimited)
      --max-steps uint          max Starlark execution steps per run, guards runaway loops (0=unlimited)
  -m, --module strings          allowed modules to preload and load (default [args,atom,base64,cache,cmd,csv,email,emoji,file,go_idiomatic,gum,hashlib,http,json,liquid,llm,log,markdown,math,net,path,qrcode,random,re,regex,runtime,serial,sqlite,stats,string,struct,sys,time,toml,totp,web,yaml])
  -o, --output string           output printer: none,stdout,stderr,basic,lineno,since,auto (default "auto")
      --record string           record the complete session output (stdout+stderr) to this transcript file
  -r, --recursion               allow recursion in Starlark code
  -V, --version                 print version & build information
  -w, --web uint16              run web server on specified port, it provides request and response structs for Starlark code to use
```

### Capabilities & sandboxing

By default StarCLI runs **open** — every wired module is available, so scripts
just work. To sandbox an untrusted script, **tighten** the capability tier with
`--caps` (or the `STAR_CAPS` env var) and a default-deny load gate is installed:

| tier | loadable modules |
|---|---|
| _(default)_ `open` | everything loadable; `cmd` loads but command **execution** stays off until `--allow-cmd` |
| `--caps full` | network **and** filesystem (but **not** `cmd`) |
| `--caps network` | safe **+** network (`http`, `net`, `email`, `llm`) |
| `--caps safe` | pure / log / process only (`math`, `json`, `sys`, `gum`, `markdown`, …) |

From a restrictive tier the granular flags widen the grant: `--allow-net`,
`--allow-fs`, and `--allow-cmd`. A module is classified by the **union** of
everything it can do, so the dual-capability modules — `web` (HTTP **+**
`static_dir`) and `sqlite` (DB **+** remote `connect_remote`) — need **both**
`--allow-net` and `--allow-fs` (or `--caps full`).

**Host command execution (`cmd`) is the sharpest tool and is gated on its own.**
The `cmd` module loads in the open posture, but `run()` is **disabled** — it
returns an error — until you pass `--allow-cmd`, which **enables execution of any
command** (no allowlist; still argv-only, never a shell). `cmd` is never granted
by a tier, not even `full`. For a one-flag "trust everything" run there is
**`--dangerously-allow-all`** — it opens network **+** filesystem **+** host
command execution of any command in a single switch. Use it only with fully
trusted scripts.

Set a stricter default for a whole deployment with the env var:

```bash
export STAR_CAPS=safe     # default every invocation to the safe tier
```

Under a restrictive tier, a script that `load()`s a withheld module fails with a
non-zero exit code (`4` for a withheld builtin). Execution budgets bound runaway
scripts: `--max-steps` caps Starlark computation steps and `--max-output` caps a
run's result size.

```bash
# open by default: anything loads
$ ./starcli -c 'load("http", "get"); print(get)'

# sandbox down to safe: a network module is now withheld
$ ./starcli --caps safe -c 'load("http", "get")'       # fails (exit 4)

# from safe, opt back into the network
$ ./starcli --caps safe --allow-net -c 'load("http", "get"); print(get)'

# host command execution needs its own explicit flag (then run() runs anything)
$ ./starcli --allow-cmd -c 'load("cmd", "run"); print(run("go version").stdout)'

# one-flag "trust everything": network + filesystem + run any command
$ ./starcli --dangerously-allow-all script.star
```

### Examples

#### REPL Mode

Start an interactive REPL session:

```bash
$ ./starcli
```

#### Execute Starlark Code Directly

Run a single line of Starlark code:

```bash
$ ./starcli -c 'print("Hello, World!")'
```

#### Execute a Script File

Run a Starlark script file:

```bash
$ ./starcli path/to/script.star
```

#### Interactive Mode After Execution

Execute code and then enter interactive mode with the environment preserved:

```bash
$ ./starcli -c 'greeting = "Hello, World!"' -i
```

#### Run as Web Server

Start a web server that executes Starlark code for HTTP requests:

```bash
$ ./starcli -w 8080 -c 'def handle_request(request): return {"message": "Hello from Starlark!"}'
```

#### Debug Mode

Run with debug-level logging:

```bash
$ ./starcli --log debug path/to/script.star
```

#### Parse Script Arguments

The `args` module is an `argparse`-style parser for the script's own arguments
(everything after `--`). `argv[0]` is the script name (or `-c`), like Python's
`sys.argv`; `parse_args()` parses `argv[1:]`.

```python
load("args", "ArgumentParser")

p = ArgumentParser(description = "greet someone")
p.add_argument("--name", default = "World", help = "who to greet")
p.add_argument("--count", type = int, default = 1)
p.add_argument("--shout", action = "store_true")
p.add_argument("file", help = "input file")

ns = p.parse_args()
print(ns.name, ns.count, ns.shout, ns.file)
```

```bash
$ ./starcli greet.star -- --name Kevin --count 3 --shout in.txt
Kevin 3 True in.txt
```

#### Capture Logs to a File

When a script uses the `log` module, `--log-file` routes all of its output to a
file at the interpreter level (the parent directory is created if needed, and
runs append):

```python
load("log", "info", "warn")
info("starting up")
warn("careful now")
```

```bash
$ ./starcli --log-file run.log job.star
$ cat run.log
2026-06-21T17:32:07.373+0800    info    starting up
2026-06-21T17:32:07.373+0800    warn    careful now
```

Use `--log-format json` for machine-readable logs (structured fields included):

```bash
$ ./starcli --log-file run.log --log-format json job.star
{"level":"info","ts":"2026-06-21T17:32:07.373+0800","msg":"starting up"}
```

#### Record a Session

`--record` saves the **complete** session output — print output, results, REPL
interaction and errors — to a transcript file (appended, with a timestamped
session header), while still showing it live. Handy for replay and review.

```bash
$ ./starcli --record session.log job.star      # works in REPL mode too
$ cat session.log

===== starcli session 2026-06-21T17:43:24+08:00 =====
... everything the run printed (stdout + stderr) ...
```

#### Read Piped Input

The `sys` module reads piped **data** from standard input (the script itself
still comes from a file or `-c`). `sys.read()` returns all of stdin; `sys.lines()`
is a **lazy** iterator (a large stream is not buffered whole); `sys.isatty()`
tells interactive from piped input.

```python
load("sys", "lines")
for line in lines():
    print(line.upper())
```

```bash
$ printf 'foo\nbar\n' | ./starcli upper.star
FOO
BAR
```

#### Check Without Running

Syntax- and resolve-check a script without executing it (reports problems as
`file:line:col: message`, non-zero exit on any problem):

```bash
$ ./starcli --check path/to/script.star
$ ./starcli --check -c 'print(undefined_name)'
direct.star:1:7: undefined: undefined_name
check: 1 problem(s) found
```

## Build your own CLI

StarCLI is the **standard, fully-loaded** build. It is assembled from a small
reusable core — the [`kit`](kit) package — that you can use to build your **own**
CLI: a few-line Go shell that embeds your Starlark scripts and wires only the
modules you need. Your shell and the standard StarCLI construct their runtime
through the same path, so they behave identically.

```go
//go:embed app.star
var app string

func main() {
	// embed the script + pick modules + run, in one call
	kit.Run(app, kit.WithModules("json", "math"))
}
```

See [`examples/`](examples) for runnable demos (a minimal shell, and one wiring a
single starpkg module) and the build-your-own quickstart.

## Configuration

StarCLI can be configured through a config file (YAML format) using the `-C` or `--config` flag:

```yaml
# Example config.yaml
host_name: MyStarCLIServer
```

## Development

### Prerequisites

- Go 1.25 or later

### Building

```bash
# Build for current platform
make build

# Build for specific platforms
make build_linux
make build_mac
make build_windows
```

### Testing

```bash
make test
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Contact

For any questions or support, please open an issue on [GitHub](https://github.com/1set/starcli/issues).
