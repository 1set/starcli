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
  -c, --code string      Starlark code to execute
  -C, --config string    config file to load
  -g, --globalreassign   allow reassigning global variables in Starlark code (default true)
  -I, --include string   include path for Starlark code to load modules from (default ".")
  -i, --interactive      enter interactive mode after executing
  -l, --log string       log level: debug, info, warn, error, dpanic, panic, fatal (default "info")
  -m, --module strings   allowed modules to preload and load (default [atom,base64,csv,email,file,go_idiomatic,gum,hashlib,http,json,llm,log,math,net,path,random,re,runtime,stats,string,struct,sys,time])
  -o, --output string    output printer: none,stdout,stderr,basic,lineno,since,auto (default "auto")
  -r, --recursion        allow recursion in Starlark code
  -V, --version          print version & build information
  -w, --web uint16       run web server on specified port, it provides request and response structs for Starlark code to use
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

- Go 1.18 or later

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
