# :stars: StarCLI

StarCLI is a command-line interface for executing Starlark scripts with various options for configuration, logging, and module inclusion.

## Installation

### Windows

Download the executable from the [releases page](https://github.com/PureMature/starcli/releases/download/b8/starcli.exe).

### macOS

Run the following commands in your terminal on an Apple Silicon Mac:

```sh
curl -sSL https://github.com/PureMature/starcli/releases/download/b8/starcli.mac_apple -o starcli
sudo install -m 0755 starcli /usr/local/bin
sudo cp -vf /usr/local/bin/starcli /usr/local/bin/starlet
```

For Intel-based Macs, use the following command:

```sh
curl -sSL https://github.com/PureMature/starcli/releases/download/b8/starcli.mac_intel -o starcli
sudo install -m 0755 starcli /usr/local/bin
sudo cp -vf /usr/local/bin/starcli /usr/local/bin/starlet
```


### Linux

Run the following commands in your terminal:

```sh
curl -sSL https://github.com/PureMature/starcli/releases/download/b8/starcli.linux -o starcli
sudo install -m 0755 starcli /usr/local/bin
sudo cp -vf /usr/local/bin/starcli /usr/local/bin/starlet
```

## Usage

Run `starcli` with the `-h` option to see a list of available commands and options.

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

- **Start REPL Mode:** `starcli`
- **Execute Starlark Code:** `starcli -c 'print("Hello, World!")'`
- **Enter Interactive Mode After Execution:** `starcli -c 's = "Hello, World!"' -i`
- **Execute Starlark Code with Log Level:** `starcli --log debug test.star`

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request for any changes.

## Contact

For any questions or support, please open an issue on [GitHub](https://github.com/PureMature/starcli/issues).
