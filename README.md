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
      --allow-cmd         widen a restrictive tier with the cmd module (host command execution)
      --allow-fs          widen a restrictive tier with filesystem modules (file, path)
      --allow-net         widen a restrictive tier with network modules (http, net, email, llm)
      --caps string       capability tier: open (default, everything) | full | network | safe; or env STAR_CAPS
      --check             syntax/resolve check the script (-c or file) without running it
  -c, --code string       Starlark code to execute
  -C, --config string     config file to load
  -g, --globalreassign    allow reassigning global variables in Starlark code (default true)
  -I, --include string    include path for Starlark code to load modules from (default ".")
  -i, --interactive       enter interactive mode after executing
  -l, --log string        log level: debug, info, warn, error, dpanic, panic, fatal (default "info")
      --max-output uint   max top-level output entries per run (0=unlimited)
      --max-steps uint    max Starlark execution steps per run, guards runaway loops (0=unlimited)
  -m, --module strings    allowed modules to preload and load (default [atom,base64,cmd,csv,email,file,go_idiomatic,gum,hashlib,http,json,llm,log,markdown,math,net,path,random,re,regex,runtime,s3,serial,sqlite,stats,string,struct,sys,time,web])
  -o, --output string     output printer: none,stdout,stderr,basic,lineno,since,auto (default "auto")
  -r, --recursion         allow recursion in Starlark code
  -V, --version           print version & build information
  -w, --web uint16        run web server on specified port, it provides request and response structs for Starlark code to use
```

### Capabilities & sandboxing

By default StarCLI runs **open** — every wired module is available, so scripts
just work. To sandbox an untrusted script, **tighten** the capability tier with
`--caps` (or the `STAR_CAPS` env var) and a default-deny load gate is installed:

| tier | loadable modules |
|---|---|
| _(default)_ `open` | everything, including `cmd` (host command execution) |
| `--caps full` | network **and** filesystem (but **not** `cmd`) |
| `--caps network` | safe **+** network (`http`, `net`, `email`, `llm`) |
| `--caps safe` | pure / log / process only (`math`, `json`, `sys`, `gum`, `markdown`, …) |

From a restrictive tier the granular flags widen the grant: `--allow-net`,
`--allow-fs`, and `--allow-cmd` (cmd is **never** granted by a tier — not even
`full` — only by `--allow-cmd`). A module is classified by the **union** of
everything it can do, so the dual-capability modules — `web` (HTTP **+**
`static_dir`), `s3` (storage **+** local file R/W), `sqlite` (DB **+** remote
`connect_remote`) — need **both** `--allow-net` and `--allow-fs` (or `--caps full`).

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

# host command execution always needs its own explicit flag
$ ./starcli --caps safe --allow-cmd -c 'load("cmd", "run"); print(run("echo", "hi"))'
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

#### Check Without Running

Syntax- and resolve-check a script without executing it (reports problems as
`file:line:col: message`, non-zero exit on any problem):

```bash
$ ./starcli --check path/to/script.star
$ ./starcli --check -c 'print(undefined_name)'
direct.star:1:7: undefined: undefined_name
check: 1 problem(s) found
```

## Configuration

StarCLI can be configured through a config file (YAML format) using the `-C` or `--config` flag:

```yaml
# Example config.yaml
host_name: MyStarCLIServer
```

## Development

### Prerequisites

- Go 1.22 or later

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
