package cli

import (
	"strings"

	"github.com/1set/starlet"
)

// Capability classification for the opt-in capability load gate (CLI-01/M2).
// starcli wires both starlet builtins and starpkg domain modules. The DEFAULT
// posture is open — every wired module is loadable — so the CLI is convenient
// out of the box. A host tightens it on purpose via --caps (safe|network|full)
// / STAR_CAPS or the granular --allow-* flags; only then is a load gate
// installed, admitting just the modules whose capabilities the grant permits.
//
// Capabilities reuse starlet.ModuleCapability bits so builtins and starpkg
// modules are judged on one scale. starlet builtins are classified by
// starlet.GetBuiltinModuleCapability; the starpkg domain modules starcli wires
// are classified here.

// modCmd is the host-command-execution module: the sharpest tool in the box. It
// is gated by --allow-cmd alone and never granted by a capability tier (not even
// "full"), so escalating to the filesystem or network never implies exec.
const modCmd = "cmd"

// starpkgCaps classifies the starpkg domain modules starcli wires. A module
// absent here falls back to starlet.GetBuiltinModuleCapability.
// A module's capability is the UNION of every builtin it exposes — a module is
// as privileged as its sharpest tool. sqlite/s3/web are dual-capability and so
// require BOTH grants (or --caps full): they each cross the net<->fs line.
var starpkgCaps = map[string]starlet.ModuleCapability{
	"args":     starlet.CapPure,                            // argparse-style parsing of the captured argv
	"markdown": starlet.CapPure,                            // goldmark render, no host effect
	"sys":      starlet.CapProcess,                         // argv/platform/host + stdin
	"gum":      starlet.CapProcess,                         // interactive terminal I/O
	"email":    starlet.CapNetwork,                         // Resend API
	"llm":      starlet.CapNetwork,                         // OpenAI-compatible API
	"web":      starlet.CapNetwork | starlet.CapFileSystem, // HTTP client + static_dir (serves a local dir)
	"s3":       starlet.CapNetwork | starlet.CapFileSystem, // object storage API + put/get_object_file (local file R/W)
	"sqlite":   starlet.CapFileSystem | starlet.CapNetwork, // local DB files + connect_remote (libsql/Turso over the network)
	modCmd:     starlet.CapProcess,                         // classified, but gated by allowCmd — see capGrant.moduleAllowed
}

// moduleCaps returns the capability set of a module (starpkg or builtin) and
// whether the name is known at all.
func moduleCaps(name string) (starlet.ModuleCapability, bool) {
	if c, ok := starpkgCaps[name]; ok {
		return c, true
	}
	return starlet.GetBuiltinModuleCapability(name)
}

// safeCaps is the most restrictive tier: pure computation (CapPure == 0),
// logging, and process/runtime info — no network, no filesystem, no exec.
const safeCaps = starlet.CapLog | starlet.CapProcess

// allCaps is every non-exec capability bit (the "open"/"full" reach).
const allCaps = safeCaps | starlet.CapNetwork | starlet.CapFileSystem

// capTiers is the set of recognised --caps / STAR_CAPS values. "" resolves to
// the default tier (open).
var capTiers = map[string]bool{"": true, "open": true, "safe": true, "network": true, "full": true}

// validCapsTier reports whether s names a recognised capability tier.
func validCapsTier(s string) bool {
	return capTiers[strings.ToLower(strings.TrimSpace(s))]
}

// capGrant is the set of capabilities a run permits, derived from the flags.
type capGrant struct {
	caps     starlet.ModuleCapability // permitted capability bits
	allowCmd bool                     // host command execution (cmd), gated alone
}

// grantFromFlags builds a capGrant from the capability flags. The tier sets a
// baseline (the default — empty or "open" — is fully open, including exec) and
// the granular --allow-* flags only widen it. Validate the tier with
// validCapsTier before calling; an unrecognised value is treated as open here.
func grantFromFlags(caps string, allowNet, allowFS, allowCmd bool) capGrant {
	g := capGrant{allowCmd: allowCmd}
	switch strings.ToLower(strings.TrimSpace(caps)) {
	case "safe":
		g.caps = safeCaps
	case "network":
		g.caps = safeCaps | starlet.CapNetwork
	case "full":
		g.caps = allCaps
	default: // "" or "open" (or, defensively, anything unrecognised) -> open
		g.caps = allCaps
		g.allowCmd = true
	}
	if allowNet {
		g.caps |= starlet.CapNetwork
	}
	if allowFS {
		g.caps |= starlet.CapFileSystem
	}
	return g
}

// unrestricted reports whether the grant permits everything (the default open
// posture). A Box built under an unrestricted grant needs no load gate at all.
func (g capGrant) unrestricted() bool {
	return g.caps == allCaps && g.allowCmd
}

// moduleAllowed reports whether a module may load under this grant. cmd is gated
// by allowCmd alone; an unknown module is denied (default deny); otherwise every
// capability bit the module needs must be granted.
func (g capGrant) moduleAllowed(name string) bool {
	if name == modCmd {
		return g.allowCmd
	}
	c, ok := moduleCaps(name)
	if !ok {
		return false
	}
	return c&^g.caps == 0
}

// allowedModules filters names to those this grant permits, preserving order.
func (g capGrant) allowedModules(names []string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if g.moduleAllowed(n) {
			out = append(out, n)
		}
	}
	return out
}
