// Separate module on purpose: these build-your-own demos each depend on only the
// runtime plus the modules they wire (not starcli's full module set), so this is
// the honest, minimal dependency tree a real shell would have. It also keeps the
// example mains out of the main module's coverage gate. CI builds it in its own
// non-gating job. The replace points at the in-repo kit until starcli is tagged.
module github.com/1set/starcli/examples

go 1.25.8

require (
	github.com/1set/starcli v0.0.0
	github.com/starpkg/qrcode v0.1.0
)

require (
	github.com/1set/starbox v0.2.0 // indirect
	github.com/1set/starlet v0.2.2 // indirect
	github.com/1set/starlight v0.2.0 // indirect
	github.com/boombuler/barcode v1.1.0 // indirect
	github.com/chzyer/readline v1.5.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/h2so5/here v0.0.0-20200815043652-5e14eb691fae // indirect
	github.com/montanaflynn/stats v0.7.1 // indirect
	github.com/psanford/memfs v0.0.0-20230130182539-4dbf7e3e865e // indirect
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1 // indirect
	github.com/spyzhov/ajson v0.9.6 // indirect
	github.com/starpkg/base v0.1.1 // indirect
	go.starlark.net v0.0.0-20260324133313-ffb3f39dd27a // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.9.0 // indirect
	go.uber.org/zap v1.24.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
)

replace github.com/1set/starcli => ../
