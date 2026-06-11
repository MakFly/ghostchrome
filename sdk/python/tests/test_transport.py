"""
Tests for SubprocessTransport.

Uses a real subprocess that echoes back JSONL lines — no ghostchrome binary needed.
Tests line-buffer reassembly, id correlation, timeout behaviour, and clean shutdown.
"""
import json
import os
import subprocess
import sys
import threading
import time
import unittest

from ghostchrome.transport import SubprocessTransport, TransportError


# ---------------------------------------------------------------------------
# Helpers: build a minimal echo-agent as a Python one-liner
# ---------------------------------------------------------------------------

ECHO_AGENT_CODE = r"""
import json, sys

for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    try:
        req = json.loads(line)
    except json.JSONDecodeError:
        continue
    resp = {"id": req.get("id", ""), "ok": True, "result": {"echo": req.get("op")}}
    sys.stdout.write(json.dumps(resp) + "\n")
    sys.stdout.flush()
    if req.get("op") == "close":
        break
"""

SLOW_AGENT_CODE = r"""
import json, sys, time

for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    try:
        req = json.loads(line)
    except json.JSONDecodeError:
        continue
    time.sleep(0.05)  # small delay
    resp = {"id": req.get("id", ""), "ok": True, "result": {}}
    sys.stdout.write(json.dumps(resp) + "\n")
    sys.stdout.flush()
    if req.get("op") == "close":
        break
"""

SPLIT_AGENT_CODE = r"""
import json, sys, time

for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    try:
        req = json.loads(line)
    except json.JSONDecodeError:
        continue
    resp = json.dumps({"id": req.get("id", ""), "ok": True, "result": {}})
    # Write the response in two chunks to test line-buffer reassembly
    half = len(resp) // 2
    sys.stdout.write(resp[:half])
    sys.stdout.flush()
    time.sleep(0.01)
    sys.stdout.write(resp[half:] + "\n")
    sys.stdout.flush()
    if req.get("op") == "close":
        break
"""


def _make_transport(agent_code: str, timeout: float = 5.0) -> SubprocessTransport:
    """Build a SubprocessTransport backed by an inline Python echo-agent."""
    return SubprocessTransport(
        command=sys.executable,
        args=["-c", agent_code],
        timeout=timeout,
    )


class TestSubprocessTransportBasic(unittest.TestCase):
    def test_send_and_receive(self):
        transport = _make_transport(ECHO_AGENT_CODE)
        try:
            resp = transport.send("navigate", {"url": "https://example.com"})
            self.assertTrue(resp["ok"])
            self.assertEqual(resp["result"]["echo"], "navigate")
        finally:
            transport.close()

    def test_id_correlation(self):
        transport = _make_transport(ECHO_AGENT_CODE)
        try:
            resp = transport.send("click", {"ref": "@1"}, req_id="custom-id-42")
            self.assertEqual(resp["id"], "custom-id-42")
        finally:
            transport.close()

    def test_auto_generated_id_is_string(self):
        transport = _make_transport(ECHO_AGENT_CODE)
        try:
            resp = transport.send("url")
            self.assertIsInstance(resp["id"], str)
            self.assertGreater(len(resp["id"]), 0)
        finally:
            transport.close()

    def test_multiple_sequential_ops(self):
        transport = _make_transport(ECHO_AGENT_CODE)
        try:
            for op in ["navigate", "extract", "click", "url"]:
                resp = transport.send(op)
                self.assertTrue(resp["ok"], f"op={op} should be ok")
                self.assertEqual(resp["result"]["echo"], op)
        finally:
            transport.close()

    def test_empty_args_sent_as_empty_dict(self):
        """Passing args=None should still work."""
        transport = _make_transport(ECHO_AGENT_CODE)
        try:
            resp = transport.send("back", None)
            self.assertTrue(resp["ok"])
        finally:
            transport.close()


class TestSubprocessTransportLinebuffer(unittest.TestCase):
    def test_split_chunk_reassembly(self):
        """
        SubprocessTransport's reader must reassemble chunks split across
        multiple reads into a complete JSON line before dispatching.
        """
        transport = _make_transport(SPLIT_AGENT_CODE)
        try:
            resp = transport.send("navigate", {"url": "https://example.com"})
            self.assertTrue(resp["ok"])
        finally:
            transport.close()

    def test_concurrent_sends(self):
        """
        Multiple threads sending ops concurrently should all get correct responses.
        Uses a slow agent to increase the chance of overlapping I/O.
        """
        transport = _make_transport(SLOW_AGENT_CODE, timeout=10.0)
        results = {}
        errors = []

        def worker(op_name):
            try:
                resp = transport.send(op_name, req_id=f"id-{op_name}")
                results[op_name] = resp
            except Exception as e:
                errors.append(e)

        threads = [threading.Thread(target=worker, args=(op,))
                   for op in ["navigate", "extract", "click"]]
        for t in threads:
            t.start()
        for t in threads:
            t.join(timeout=10)

        transport.close()
        self.assertEqual(errors, [], f"Unexpected errors: {errors}")
        for op in ["navigate", "extract", "click"]:
            self.assertIn(op, results)
            self.assertEqual(results[op]["id"], f"id-{op}")
            self.assertTrue(results[op]["ok"])


class TestSubprocessTransportShutdown(unittest.TestCase):
    def test_close_is_idempotent(self):
        transport = _make_transport(ECHO_AGENT_CODE)
        transport.close()
        transport.close()  # should not raise

    def test_send_after_close_raises(self):
        transport = _make_transport(ECHO_AGENT_CODE)
        transport.close()
        with self.assertRaises(TransportError):
            transport.send("navigate")


class TestSubprocessTransportTimeout(unittest.TestCase):
    def test_timeout_raises_transport_error(self):
        """
        A transport with a very short timeout against a never-responding agent
        should raise TransportError with a timeout message.
        """
        # A process that reads but never writes back
        silent_code = "import sys\nfor line in sys.stdin:\n pass\n"
        transport = _make_transport(silent_code, timeout=0.2)
        try:
            with self.assertRaises(TransportError) as ctx:
                transport.send("navigate")
            self.assertIn("timeout", str(ctx.exception).lower())
        finally:
            try:
                transport._proc.kill()
                transport._proc.wait()
            except Exception:
                pass


if __name__ == "__main__":
    unittest.main()
