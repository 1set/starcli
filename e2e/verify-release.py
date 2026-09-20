#!/usr/bin/env python3
"""Verify real release archives, then rehearse install, upgrade, and rollback.

Usage: python3 e2e/verify-release.py DIST VERSION PREVIOUS_DIST PREVIOUS_VERSION
Only a temporary installation directory is changed. No system install or
release publication occurs. Run from the repository root with Go on PATH.
"""

import hashlib
import io
import os
from pathlib import Path
import platform
import re
import subprocess
import sys
import tarfile
import tempfile
import zipfile


TARGETS = {
    ("linux", "amd64"), ("linux", "arm64"),
    ("macOS", "amd64"), ("macOS", "arm64"),
    ("windows", "amd64"), ("windows", "arm64"),
}


def native_target():
    system = {"Linux": "linux", "Darwin": "macOS", "Windows": "windows"}[platform.system()]
    arch = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}[platform.machine().lower()]
    return system, arch


def verify_archives(directory, version, *, native_only=False):
    """Pin the complete archive/checksum/member contract for all six targets."""
    if not re.fullmatch(r"\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?", version):
        raise ValueError(f"invalid version: {version!r}")
    expected = {
        f"starcli_{version}_{system}_{arch}." + ("zip" if system == "windows" else "tar.gz"): (system, arch)
        for system, arch in TARGETS
    }
    checksums = {}
    for line in (directory / "checksums.txt").read_text().splitlines():
        match = re.fullmatch(r"([a-f0-9]{64})  (\S+)", line)
        if not match or match[2] in checksums:
            raise ValueError("invalid or duplicate checksum entry")
        checksums[match[2]] = match[1]
    if checksums.keys() != expected.keys():
        raise ValueError("checksum manifest must contain exactly the six supported archives")
    target = native_target()
    archives = {name for name, value in expected.items() if not native_only or value == target}
    actual = {p.name for p in directory.iterdir() if p.is_file()}
    if actual != archives | {"checksums.txt"}:
        raise ValueError(f"unexpected or missing distribution files: {actual ^ (archives | {'checksums.txt'})}")
    native_binary = None
    for name in sorted(archives):
        archive = (directory / name).read_bytes()
        if hashlib.sha256(archive).hexdigest() != checksums[name]:
            raise ValueError(f"checksum mismatch: {name}")
        system, arch = expected[name]
        binary = "starcli.exe" if system == "windows" else "starcli"
        members = {}
        if system == "windows":
            with zipfile.ZipFile(io.BytesIO(archive)) as package:
                for member in package.infolist():
                    if member.is_dir() or member.filename in members:
                        raise ValueError(f"non-file or duplicate ZIP member: {member.filename}")
                    members[member.filename] = package.read(member)
        else:
            with tarfile.open(fileobj=io.BytesIO(archive), mode="r:gz") as package:
                for member in package:
                    if not member.isfile() or member.name in members:
                        raise ValueError(f"non-file or duplicate TAR member: {member.name}")
                    if member.name == binary and not member.mode & 0o111:
                        raise ValueError(f"binary is not executable: {name}")
                    members[member.name] = package.extractfile(member).read()
        if members.keys() != {binary, "README.md", "LICENSE"} or not all(members.values()):
            raise ValueError(f"archive must contain only a binary, README and license: {name}")
        if (system, arch) == target:
            native_binary = members[binary]
        print(f"verified {name}", flush=True)
    if native_binary is None:
        raise ValueError(f"missing native binary: {target}")
    return native_binary


def run(binary, *args, expected_stdout=None, success=True):
    result = subprocess.run([str(binary), *args], capture_output=True, text=True, timeout=30)
    if (result.returncode == 0) != success or (expected_stdout is not None and result.stdout != expected_stdout):
        raise RuntimeError(f"{args!r}: exit={result.returncode}\n{result.stdout}\n{result.stderr}")
    return result.stdout


def smoke(binary, version):
    banner = re.sub(r"\x1b\[[0-9;]*m", "", run(binary, "--version"))
    # v0.1.2 printed a bare version; current releases include the v prefix.
    summaries = [line.split("GitSummary: ", 1)[1] for line in banner.splitlines() if "GitSummary: " in line]
    if len(summaries) != 1 or summaries[0].removeprefix("v") != version:
        raise ValueError(f"installed binary does not identify v{version}: {banner}")
    run(binary, "-c", "print(6 * 7)", expected_stdout="42\n")
    run(binary, "-c", 'fail("installation test")', success=False)


def verify_terminal_recording(binary, directory):
    if os.name != "posix":
        print("PTY recording test requires Unix; portable recording is covered by e2e", flush=True)
        return
    import errno
    import pty
    import select
    import signal
    import termios
    import time

    for mode, flags in (("repl", []), ("inspect", ["-i", "-c", "value = 40"])):
        transcript = directory / (mode + ".log")
        pid, fd = pty.fork()
        if pid == 0:
            os.environ["TERM"] = "xterm-256color"
            os.execv(str(binary), [str(binary), "--caps", "safe", "--record", str(transcript), *flags])
        output = bytearray()

        def read_output(timeout):
            if not select.select([fd], [], [], timeout)[0]:
                return
            try:
                output.extend(os.read(fd, 65536))
            except OSError as error:
                # Linux reports EIO when the child closes its terminal.
                if error.errno != errno.EIO:
                    raise
            if len(output) > 1024 * 1024:
                raise RuntimeError("unexpectedly large terminal output")

        def receive(token, start=0):
            deadline = time.monotonic() + 10
            while time.monotonic() < deadline:
                # readline leaves raw mode while evaluating a command. Wait
                # for the next read before sending control keys: canonical
                # terminal handling can otherwise consume EOF prematurely.
                if token in output[start:] and not termios.tcgetattr(fd)[3] & termios.ICANON:
                    return
                read_output(0.1)
            raise RuntimeError(f"missing terminal output {token!r}: {bytes(output)!r}")

        try:
            receive(b">>> ")
            for source, expected in (
                (b'print("recorded " + str(6*7))\n', b"recorded 42"),
                (b'fail("terminal failure")\n', b"fail: terminal failure"),
                (b'"recovered " + str(6*7)\n', b"recovered 42"),
                (b'\x03', b"Interrupt"),
                (b'print("after interrupt " + str(6*7))\n', b"after interrupt 42"),
            ):
                start = len(output)
                os.write(fd, source)
                receive(expected, start)
            os.write(fd, b"\x04")
            deadline = time.monotonic() + 10
            while time.monotonic() < deadline:
                child, status = os.waitpid(pid, os.WNOHANG)
                if child:
                    pid = None
                    if os.waitstatus_to_exitcode(status) != 0:
                        raise RuntimeError(f"terminal session failed: {status}")
                    break
                # Drain the terminal until exit, including final prompt/echo
                # bytes, so the child cannot block on its output buffer.
                read_output(0.05)
            else:
                raise RuntimeError(f"terminal session did not exit after EOF: {bytes(output[-4096:])!r}")
            data = transcript.read_text()
            for expected in (">>> ", "recorded 42", "fail: terminal failure", "recovered 42", "after interrupt 42"):
                if expected not in data:
                    raise RuntimeError(f"transcript omitted {expected!r}")
            print(f"passed real terminal recording: {mode}", flush=True)
        finally:
            os.close(fd)
            if pid is not None:
                os.kill(pid, signal.SIGKILL)
                os.waitpid(pid, 0)


def main():
    directory, version, previous, previous_version = sys.argv[1:]
    candidate = verify_archives(Path(directory), version)
    baseline = verify_archives(Path(previous), previous_version, native_only=True)
    with tempfile.TemporaryDirectory(prefix="starcli install with spaces ") as temp:
        binary = Path(temp) / ("starcli.exe" if os.name == "nt" else "starcli")
        for label, data, expected_version in (
            ("install baseline", baseline, previous_version),
            ("upgrade candidate", candidate, version),
            ("rollback baseline", baseline, previous_version),
        ):
            pending = binary.with_suffix(".pending")
            pending.write_bytes(data)
            pending.chmod(0o755)
            pending.replace(binary)
            smoke(binary, expected_version)
            if label == "upgrade candidate":
                subprocess.run(
                    ["go", "test", "-v", "./e2e", "-count=1"], check=True,
                    env={**os.environ, "STARCLI_TEST_BINARY": str(binary)}, timeout=180,
                )
                verify_terminal_recording(binary, Path(temp))
            if binary.read_bytes() != data:
                raise ValueError(f"installed bytes changed during {label}")
            print(f"passed {label}: v{expected_version}", flush=True)


if __name__ == "__main__":
    main()
