"""Tests for ghostchrome.types — parse_response, Observation, result types."""
import json
import unittest

from ghostchrome.types import (
    ConsoleError,
    NetworkError,
    Observation,
    Response,
    _parse_observation,
    parse_response,
    _parse_snapshot_diff,
    SnapshotDiff,
)


class TestParseObservation(unittest.TestCase):
    def test_none_returns_none(self):
        self.assertIsNone(_parse_observation(None))

    def test_empty_dict(self):
        obs = _parse_observation({})
        self.assertIsInstance(obs, Observation)
        self.assertIsNone(obs.url)
        self.assertEqual(obs.console_errors, [])
        self.assertEqual(obs.network_failed, [])
        self.assertIsNone(obs.a11y_diff)
        self.assertIsNone(obs.dialog)
        self.assertIsNone(obs.captcha_hint)

    def test_full_observation(self):
        raw = {
            "url": "https://example.com/page",
            "console_errors": [
                {"level": "error", "text": "TypeError: x", "source": "app.js:10:1"}
            ],
            "network_failed": [
                {"url": "https://api.example.com/me", "status": 401, "failed": ""}
            ],
            "a11y_diff": "added 2 nodes",
            "dialog": "Are you sure?",
            "captcha_hint": "DataDome detected",
        }
        obs = _parse_observation(raw)
        self.assertEqual(obs.url, "https://example.com/page")
        self.assertEqual(len(obs.console_errors), 1)
        self.assertIsInstance(obs.console_errors[0], ConsoleError)
        self.assertEqual(obs.console_errors[0].level, "error")
        self.assertEqual(obs.console_errors[0].text, "TypeError: x")
        self.assertEqual(obs.console_errors[0].source, "app.js:10:1")
        self.assertEqual(len(obs.network_failed), 1)
        self.assertIsInstance(obs.network_failed[0], NetworkError)
        self.assertEqual(obs.network_failed[0].status, 401)
        self.assertEqual(obs.a11y_diff, "added 2 nodes")
        self.assertEqual(obs.dialog, "Are you sure?")
        self.assertEqual(obs.captcha_hint, "DataDome detected")


class TestParseResponse(unittest.TestCase):
    def test_ok_response(self):
        line = json.dumps({
            "id": "r1",
            "ok": True,
            "result": {"url": "https://example.com", "status": 200, "title": "Example"},
            "observation": {"url": "https://example.com"},
        })
        resp = parse_response(line)
        self.assertIsInstance(resp, Response)
        self.assertEqual(resp.id, "r1")
        self.assertTrue(resp.ok)
        self.assertEqual(resp.result["status"], 200)
        self.assertIsNotNone(resp.observation)
        self.assertEqual(resp.observation.url, "https://example.com")

    def test_error_response(self):
        line = json.dumps({"id": "r2", "ok": False, "error": "element not found"})
        resp = parse_response(line)
        self.assertFalse(resp.ok)
        self.assertEqual(resp.error, "element not found")
        # result is None when omitted (not {}) — caller must handle both
        self.assertIsNone(resp.result)
        self.assertIsNone(resp.observation)

    def test_no_observation(self):
        line = json.dumps({"id": "r3", "ok": True, "result": {}})
        resp = parse_response(line)
        self.assertIsNone(resp.observation)

    def test_omitted_result_is_none(self):
        """ops that return nothing omit 'result' entirely; parse_response gives None."""
        line = json.dumps({"id": "r4", "ok": True})
        resp = parse_response(line)
        self.assertIsNone(resp.result)

    def test_list_result_preserved(self):
        """errors op returns a list directly, not a dict."""
        line = json.dumps({
            "id": "r5",
            "ok": True,
            "result": [
                {"type": "console", "level": "error", "message": "oops",
                 "source": "app.js:1", "time_ms": 10},
            ],
        })
        resp = parse_response(line)
        self.assertIsInstance(resp.result, list)
        self.assertEqual(len(resp.result), 1)

    def test_events_at_envelope_level(self):
        """events is a top-level envelope field, not inside observation."""
        line = json.dumps({
            "id": "r6",
            "ok": True,
            "result": {},
            "events": [{"type": "dialog", "msg": "Are you sure?"}],
        })
        resp = parse_response(line)
        self.assertEqual(len(resp.events), 1)
        self.assertEqual(resp.events[0]["type"], "dialog")

    def test_agent_metadata_is_preserved(self):
        resp = parse_response(json.dumps({
            "id": "r7", "ok": True, "protocol": 1,
            "error_code": "x", "retryable": False,
        }))
        self.assertEqual(resp.protocol, 1)
        self.assertEqual(resp.error_code, "x")
        self.assertFalse(resp.retryable)

    def test_snapshot_diff(self):
        diff = _parse_snapshot_diff({
            "unchanged": False,
            "added": [{"ref": "@3", "role": "button", "name": "Go"}],
            "removed": ["@2"],
            "changed": {"@1": {"before": {"name": "A"}, "after": {"name": "B"}}},
            "stats": {"added": 1, "removed": 1, "changed": 1, "kept": 4},
        })
        self.assertIsInstance(diff, SnapshotDiff)
        self.assertEqual(diff.added[0].ref, "@3")
        self.assertEqual(diff.changed["@1"].after.name, "B")
        self.assertEqual(diff.stats.kept, 4)

    def test_mutation_result_full_extract(self):
        from ghostchrome.types import ExtractResult, _parse_mutation_result
        result = _parse_mutation_result({
            "nodes": [{"ref": "@1", "role": "button", "name": "Go"}],
            "refs": {"@1": {"role": "button", "name": "Go"}},
            "stats": {"total_nodes": 1, "filtered_nodes": 1, "interactive_count": 1},
        })
        self.assertIsInstance(result, ExtractResult)
        self.assertEqual(result.stats.interactive_count, 1)

    def test_mutation_result_diff(self):
        from ghostchrome.types import SnapshotDiff, _parse_mutation_result
        result = _parse_mutation_result({"unchanged": True, "stats": {"kept": 2}})
        self.assertIsInstance(result, SnapshotDiff)
        self.assertTrue(result.unchanged)


if __name__ == "__main__":
    unittest.main()
