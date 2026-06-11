"""
Contract coverage test.

Reads contracts/commands.json and asserts that every op with surface "jsonl"
has a corresponding method on the Ghostchrome client.

The op-to-method mapping accounts for Python name-mangling:
  - type  → type_
  - eval  → eval_
  - close → close (lifecycle, not a user-facing method per se, but it exists)
"""
import json
import os
import unittest

from ghostchrome.client import Ghostchrome


# Ops that are present in the contract but intentionally not exposed as a
# named method (they are driven internally by the client lifecycle).
_INTERNAL_OPS: set[str] = set()

# Python method name overrides where the op name collides with a builtin.
_OP_TO_METHOD: dict[str, str] = {
    "type": "type_",
    "eval": "eval_",
}


def _contract_path() -> str:
    """Locate contracts/commands.json relative to this test file."""
    here = os.path.dirname(os.path.abspath(__file__))
    # sdk/python/tests/ → repo root is three levels up
    repo_root = os.path.normpath(os.path.join(here, "..", "..", ".."))
    return os.path.join(repo_root, "contracts", "commands.json")


class TestContractCoverage(unittest.TestCase):
    """Every JSONL-surface op must have a method on Ghostchrome."""

    @classmethod
    def setUpClass(cls) -> None:
        path = _contract_path()
        if not os.path.exists(path):
            raise unittest.SkipTest(f"contracts/commands.json not found at {path!r}")
        with open(path) as f:
            cls.commands = json.load(f)

    def test_jsonl_ops_have_client_methods(self):
        jsonl_ops = [
            cmd["name"]
            for cmd in self.commands
            if "jsonl" in cmd.get("surfaces", [])
        ]
        self.assertGreater(len(jsonl_ops), 0, "no JSONL ops found in contract")

        missing = []
        for op in jsonl_ops:
            if op in _INTERNAL_OPS:
                continue
            method_name = _OP_TO_METHOD.get(op, op)
            # scroll_by / scroll_to use underscores — contract uses those names
            if not hasattr(Ghostchrome, method_name):
                missing.append(f"{op!r} (expected method {method_name!r})")

        self.assertEqual(
            missing,
            [],
            "JSONL ops missing client methods:\n  " + "\n  ".join(missing),
        )

    def test_all_jsonl_ops_listed(self):
        """Snapshot the set of JSONL ops so drift is caught immediately."""
        jsonl_ops = sorted(
            cmd["name"]
            for cmd in self.commands
            if "jsonl" in cmd.get("surfaces", [])
        )
        # Must include at minimum the core ops
        core = {"navigate", "extract", "click", "type", "press", "url", "errors",
                "screenshot", "eval", "scroll_by", "scroll_to", "wait", "close",
                "init", "back", "forward", "hover", "select", "fill"}
        for op in core:
            self.assertIn(op, jsonl_ops, f"core op {op!r} missing from contract")


if __name__ == "__main__":
    unittest.main()
