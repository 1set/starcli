"""Exercise the archive-to-scanner boundary without fetching external tools."""
import hashlib
import importlib.util
import io
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import unittest
from unittest import mock
import zipfile


SPEC = importlib.util.spec_from_file_location("verify_release", Path(__file__).with_name("verify-release.py"))
release = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(release)


class ArchiveScanTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.dist = Path(self.temp.name)
        self.version = "0.1.3"
        self.binaries = {}
        hashes = []
        for system, arch in sorted(release.TARGETS):
            binary = "starcli.exe" if system == "windows" else "starcli"
            data = f"fixture binary {system}/{arch}".encode()
            self.binaries[(system, arch)] = data
            name = f"starcli_{self.version}_{system}_{arch}." + ("zip" if system == "windows" else "tar.gz")
            members = {binary: data, "README.md": b"usage", "LICENSE": b"license", "SECURITY.md": b"policy"}
            path = self.dist / name
            if system == "windows":
                with zipfile.ZipFile(path, "w") as package:
                    for member, contents in members.items():
                        package.writestr(member, contents)
            else:
                with tarfile.open(path, "w:gz") as package:
                    for member, contents in members.items():
                        info = tarfile.TarInfo(member)
                        info.size = len(contents)
                        info.mode = 0o755 if member == binary else 0o644
                        package.addfile(info, io.BytesIO(contents))
            hashes.append(f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {name}\n")
        (self.dist / "checksums.txt").write_text("".join(hashes))

    def test_all_six_original_binaries_reach_pinned_scanner(self):
        scanned = []

        def scanner(argv, **kwargs):
            self.assertEqual(argv[:4], ["go", "run", "golang.org/x/vuln/cmd/govulncheck@v1.8.0", "-mode=binary"])
            self.assertEqual(kwargs, {"check": True, "timeout": 180})
            scanned.append(Path(argv[-1]).read_bytes())
            return subprocess.CompletedProcess(argv, 0)

        with mock.patch.object(release.subprocess, "run", side_effect=scanner):
            release.scan_archives(self.dist, self.version)
        self.assertCountEqual(scanned, self.binaries.values())

    def test_scanner_failure_stops_verification(self):
        with mock.patch.object(release.subprocess, "run", side_effect=subprocess.CalledProcessError(3, "scanner")) as scanner:
            with self.assertRaises(subprocess.CalledProcessError):
                release.scan_archives(self.dist, self.version)
        self.assertEqual(scanner.call_count, 1)

    def test_bad_checksum_stops_before_scanner(self):
        last = sorted(p for p in self.dist.iterdir() if p.name != "checksums.txt")[-1]
        last.write_bytes(b"tampered archive")
        with mock.patch.object(release.subprocess, "run") as scanner:
            with self.assertRaisesRegex(ValueError, "checksum mismatch"):
                release.scan_archives(self.dist, self.version)
        scanner.assert_not_called()

    def test_goreleaser_metadata_does_not_change_uploaded_asset_contract(self):
        for name in ["metadata.json", "artifacts.json", "config.yaml"]:
            (self.dist / name).write_text("metadata")
        (self.dist / "starcli_linux_amd64_v1").mkdir()
        (self.dist / "starcli_linux_amd64_v1" / "starcli").write_bytes(b"build intermediate")
        with mock.patch.object(release.subprocess, "run") as scanner:
            release.scan_archives(self.dist, self.version)
        self.assertEqual(scanner.call_count, 6)
        with self.assertRaisesRegex(ValueError, "unexpected or missing"):
            release.verify_archives(self.dist, self.version)

    def test_bad_last_member_stops_before_scanner_after_checksum_passes(self):
        last = sorted(p for p in self.dist.iterdir() if p.name != "checksums.txt")[-1]
        with zipfile.ZipFile(last, "w") as package:
            package.writestr("starcli.exe", b"binary")
            package.writestr("README.md", b"usage")
            package.writestr("LICENSE", b"license")
            package.writestr("unexpected.txt", b"missing SECURITY.md")
        manifest = self.dist / "checksums.txt"
        manifest.write_text("".join(
            f"{hashlib.sha256((self.dist / line.split()[1]).read_bytes()).hexdigest()}  {line.split()[1]}\n"
            for line in manifest.read_text().splitlines()
        ))
        with mock.patch.object(release.subprocess, "run") as scanner:
            with self.assertRaisesRegex(ValueError, "archive must contain exactly"):
                release.scan_archives(self.dist, self.version)
        scanner.assert_not_called()

    def test_extra_archive_is_rejected_even_in_scan_mode(self):
        (self.dist / "extra.zip").write_bytes(b"extra archive")
        with mock.patch.object(release.subprocess, "run") as scanner:
            with self.assertRaisesRegex(ValueError, "unexpected or missing"):
                release.scan_archives(self.dist, self.version)
        scanner.assert_not_called()

    def test_install_archive_api_and_scan_dispatch(self):
        self.assertEqual(release.verify_archives(self.dist, self.version), self.binaries[release.native_target()])
        with mock.patch.object(sys, "argv", ["verify-release.py", "--scan-binaries", str(self.dist), self.version]):
            with mock.patch.object(release, "scan_archives") as scan:
                release.main()
        scan.assert_called_once_with(self.dist, self.version)


if __name__ == "__main__":
    unittest.main()
