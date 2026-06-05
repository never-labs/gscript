import os
import shutil
import stat
import subprocess
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / "scripts" / "editor_check.sh"


class EditorCheckScriptTest(unittest.TestCase):
    def run_editor_check(self, *args, tree_sitter_stub=None):
        with tempfile.TemporaryDirectory() as td:
            bin_dir = Path(td) / "bin"
            bin_dir.mkdir()
            for tool in ("python3", "node"):
                target = shutil.which(tool)
                if not target:
                    self.skipTest(f"{tool} is required to exercise editor_check.sh")
                (bin_dir / tool).symlink_to(target)
            if tree_sitter_stub is not None:
                tree_sitter = bin_dir / "tree-sitter"
                tree_sitter.write_text(tree_sitter_stub, encoding="utf-8")
                tree_sitter.chmod(tree_sitter.stat().st_mode | stat.S_IXUSR)

            env = os.environ.copy()
            env["PATH"] = f"{bin_dir}{os.pathsep}/usr/bin{os.pathsep}/bin"
            return subprocess.run(
                ["bash", str(SCRIPT), *args],
                cwd=ROOT,
                env=env,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                text=True,
                check=False,
            )

    def test_unknown_argument_exits_with_usage(self):
        proc = self.run_editor_check("--bad-flag")

        self.assertEqual(proc.returncode, 2, proc.stdout)
        self.assertIn("unknown argument: --bad-flag", proc.stdout)
        self.assertIn("Usage: scripts/editor_check.sh", proc.stdout)

    def test_require_tree_sitter_fails_when_cli_is_unavailable(self):
        local_cli = ROOT / "tools" / "tree-sitter-leia" / "node_modules" / ".bin" / "tree-sitter"
        if local_cli.exists():
            self.skipTest("local tree-sitter CLI is installed")

        proc = self.run_editor_check("--require-tree-sitter")

        self.assertEqual(proc.returncode, 1, proc.stdout)
        self.assertIn("tree-sitter CLI is required", proc.stdout)
        self.assertIn("npm --prefix tools/tree-sitter-leia ci", proc.stdout)

    def test_require_tree_sitter_accepts_tree_sitter_on_path(self):
        proc = self.run_editor_check(
            "--require-tree-sitter",
            tree_sitter_stub="#!/usr/bin/env sh\n[ \"$1\" = test ] || exit 64\nexit 0\n",
        )

        self.assertEqual(proc.returncode, 0, proc.stdout)
        self.assertIn("editor_check.sh: ok", proc.stdout)


if __name__ == "__main__":
    unittest.main()
