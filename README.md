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
      --allow-cmd         allow the cmd module to execute host commands (never granted by a tier)
      --allow-fs          allow modules that touch the filesystem (sqlite, file, path)
      --allow-net         allow modules that open network connections (email, llm, web, s3, http, net)
      --caps string       capability tier: safe (default, no net/fs/cmd), network, full (default "safe")
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

StarCLI runs scripts behind a **default-deny capability gate**: by default only
the **Safe** tier loads — pure computation, logging, and process/runtime info
(`math`, `json`, `string`, `sys`, `gum`, `markdown`, …). Modules that reach the
network, the filesystem, or the host shell are **withheld** until you opt in:

| flag | grants |
|---|---|
| _(default)_ `--caps safe` | pure / log / process modules only |
| `--allow-net` (or `--caps network`) | network modules: `http`, `net`, `email`, `llm` |
| `--allow-fs` | filesystem modules: `file`, `path` |
| `--caps full` | network **and** filesystem (but **not** `cmd`) |
| `--allow-cmd` | the `cmd` module (host command execution) — never granted by a tier, even `full` |

A module is classified by the **union** of everything it can do, so the
dual-capability modules — `web` (HTTP client **+** `static_dir`), `s3` (object
storage **+** local file read/write), and `sqlite` (local DB **+** remote
`connect_remote`) — require **both** `--allow-net` **and** `--allow-fs` (or just
`--caps full`).

A script that `load()`s a withheld module fails with a non-zero exit code (`4`
for a withheld builtin). Execution budgets bound runaway scripts: `--max-steps`
caps Starlark computation steps and `--max-output` caps a run's result size.

```bash
# default Safe: a network module is withheld
$ ./starcli -c 'load("http", "get")'       # fails (exit 4)

# opt in to the network
$ ./starcli --allow-net -c 'load("http", "get"); print(get)'

# web/s3/sqlite span net + fs, so they need both grants (or --caps full)
$ ./starcli --caps full -c 'load("sqlite", "connect"); print(connect)'

# host command execution requires its own explicit flag
$ ./starcli --allow-cmd -c 'load("cmd", "run"); print(run("echo", "hi"))'
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
