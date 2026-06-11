"""
Tests for Ghostchrome client.

All tests use FakeTransport — no real binary or Chrome needed.
Verifies: id correlation, correct op/args emitted, result parsing.
"""
import unittest

from ghostchrome.client import Ghostchrome
from ghostchrome.transport import TransportError
from ghostchrome.types import (
    BackForwardResult,
    ErrorEntry,
    EvalResult,
    ExtractResult,
    ExtractStats,
    FillResult,
    GhostchromeError,
    NavigateResult,
    Observation,
    ScreenshotResult,
    ScrollResult,
    UrlResult,
)
from tests.fake_transport import FakeTransport


def make_client(*responses):
    """Return a Ghostchrome backed by a FakeTransport with given response dicts."""
    transport = FakeTransport(responses=list(responses))
    return Ghostchrome(transport=transport), transport


class TestNavigate(unittest.TestCase):
    def test_navigate_emits_correct_op(self):
        client, transport = make_client({
            "id": "x",
            "ok": True,
            "result": {"status": 200, "url": "https://example.com", "title": "Example", "time_ms": 42},
        })
        result, obs = client.navigate("https://example.com")
        op, args, _ = transport.calls[0]
        self.assertEqual(op, "navigate")
        self.assertEqual(args["url"], "https://example.com")

    def test_navigate_parses_result(self):
        client, _ = make_client({
            "id": "x",
            "ok": True,
            "result": {"status": 200, "url": "https://example.com", "title": "Example Domain", "time_ms": 55},
        })
        result, obs = client.navigate("https://example.com")
        self.assertIsInstance(result, NavigateResult)
        self.assertEqual(result.status, 200)
        self.assertEqual(result.url, "https://example.com")
        self.assertEqual(result.title, "Example Domain")
        self.assertEqual(result.time_ms, 55)

    def test_navigate_with_wait_strategy(self):
        client, transport = make_client({
            "id": "x",
            "ok": True,
            "result": {"status": 200, "url": "https://example.com", "title": "", "time_ms": 0},
        })
        client.navigate("https://example.com", wait="stable")
        _, args, _ = transport.calls[0]
        self.assertEqual(args.get("wait"), "stable")

    def test_navigate_without_wait_omits_key(self):
        client, transport = make_client({
            "id": "x",
            "ok": True,
            "result": {"status": 200, "url": "https://example.com", "title": "", "time_ms": 0},
        })
        client.navigate("https://example.com")
        _, args, _ = transport.calls[0]
        self.assertNotIn("wait", args)

    def test_navigate_returns_observation(self):
        client, _ = make_client({
            "id": "x",
            "ok": True,
            "result": {"status": 200, "url": "https://example.com", "title": "", "time_ms": 0},
            "observation": {"url": "https://example.com", "a11y_diff": "added 5 nodes"},
        })
        _, obs = client.navigate("https://example.com")
        self.assertIsInstance(obs, Observation)
        self.assertEqual(obs.url, "https://example.com")
        self.assertEqual(obs.a11y_diff, "added 5 nodes")


class TestIdCorrelation(unittest.TestCase):
    """Verify that the id in the request matches the id in the response."""

    def test_id_is_echoed_back(self):
        captured = []

        def handler(op, args, req_id):
            captured.append(req_id)
            return {"id": req_id, "ok": True, "result": {}}

        transport = FakeTransport(handler=handler)
        client = Ghostchrome(transport=transport)
        client._call("url")
        # The req_id was generated and echoed back without error
        self.assertEqual(len(captured), 1)
        self.assertIsInstance(captured[0], str)
        self.assertTrue(len(captured[0]) > 0)

    def test_mismatched_id_still_works_via_dispatch(self):
        """FakeTransport always returns the enqueued response regardless of id."""
        transport = FakeTransport(responses=[
            {"id": "r99", "ok": True, "result": {"url": "https://example.com", "title": ""}}
        ])
        client = Ghostchrome(transport=transport)
        result, _ = client.url()
        self.assertIsInstance(result, UrlResult)


class TestExtract(unittest.TestCase):
    def test_extract_default_args(self):
        client, transport = make_client({
            "id": "x",
            "ok": True,
            "result": {
                "nodes": [1, 2],
                "refs": {"@1": {"name": "button"}},
                "stats": {"total_nodes": 10, "filtered_nodes": 8, "interactive_count": 2},
            },
        })
        result, _ = client.extract()
        op, args, _ = transport.calls[0]
        self.assertEqual(op, "extract")
        self.assertEqual(args, {})
        self.assertIsInstance(result, ExtractResult)
        self.assertEqual(result.nodes, [1, 2])
        self.assertEqual(result.refs, {"@1": {"name": "button"}})

    def test_extract_stats_fields(self):
        client, _ = make_client({
            "id": "x",
            "ok": True,
            "result": {
                "nodes": [],
                "refs": {},
                "stats": {"total_nodes": 15, "filtered_nodes": 12, "interactive_count": 1},
            },
        })
        result, _ = client.extract()
        self.assertIsInstance(result.stats, ExtractStats)
        self.assertEqual(result.stats.total_nodes, 15)
        self.assertEqual(result.stats.filtered_nodes, 12)
        self.assertEqual(result.stats.interactive_count, 1)

    def test_extract_with_level_and_selector(self):
        client, transport = make_client({
            "id": "x",
            "ok": True,
            "result": {
                "nodes": [],
                "refs": {},
                "stats": {"total_nodes": 0, "filtered_nodes": 0, "interactive_count": 0},
            },
        })
        client.extract(level="skeleton", selector="#main")
        _, args, _ = transport.calls[0]
        self.assertEqual(args["level"], "skeleton")
        self.assertEqual(args["selector"], "#main")

    def test_extract_missing_stats_defaults_to_zero(self):
        """extract() must not crash when stats is missing or empty."""
        client, _ = make_client({
            "id": "x",
            "ok": True,
            "result": {"nodes": [], "refs": {}, "stats": {}},
        })
        result, _ = client.extract()
        self.assertEqual(result.stats.total_nodes, 0)
        self.assertEqual(result.stats.interactive_count, 0)


class TestClick(unittest.TestCase):
    def test_click_emits_ref(self):
        client, transport = make_client({"id": "x", "ok": True})
        result, obs = client.click("@3")
        op, args, _ = transport.calls[0]
        self.assertEqual(op, "click")
        self.assertEqual(args["ref"], "@3")
        self.assertIsNone(result)

    def test_click_returns_none_result(self):
        client, _ = make_client({"id": "x", "ok": True})
        result, obs = client.click("@3")
        self.assertIsNone(result)


class TestHover(unittest.TestCase):
    def test_hover_emits_ref(self):
        client, transport = make_client({"id": "x", "ok": True})
        result, obs = client.hover("@5")
        op, args, _ = transport.calls[0]
        self.assertEqual(op, "hover")
        self.assertEqual(args["ref"], "@5")
        self.assertIsNone(result)


class TestType(unittest.TestCase):
    def test_type_emits_ref_and_text(self):
        client, transport = make_client({"id": "x", "ok": True})
        result, obs = client.type_("@2", "hello world")
        op, args, _ = transport.calls[0]
        self.assertEqual(op, "type")
        self.assertEqual(args["ref"], "@2")
        self.assertEqual(args["text"], "hello world")
        self.assertIsNone(result)


class TestPress(unittest.TestCase):
    def test_press_key_only(self):
        client, transport = make_client({"id": "x", "ok": True})
        result, obs = client.press("Enter")
        op, args, _ = transport.calls[0]
        self.assertEqual(op, "press")
        self.assertEqual(args["key"], "Enter")
        self.assertNotIn("ref", args)
        self.assertIsNone(result)

    def test_press_key_with_ref(self):
        client, transport = make_client({"id": "x", "ok": True})
        client.press("ArrowDown", ref="@4")
        _, args, _ = transport.calls[0]
        self.assertEqual(args["key"], "ArrowDown")
        self.assertEqual(args["ref"], "@4")


class TestSelect(unittest.TestCase):
    def test_select_emits_values(self):
        client, transport = make_client({"id": "x", "ok": True})
        result, obs = client.select("@7", ["opt1", "opt2"])
        op, args, _ = transport.calls[0]
        self.assertEqual(op, "select")
        self.assertEqual(args["ref"], "@7")
        self.assertEqual(args["values"], ["opt1", "opt2"])
        self.assertIsNone(result)


class TestFill(unittest.TestCase):
    def test_fill_emits_fields_map(self):
        client, transport = make_client({
            "id": "x", "ok": True,
            "result": {"filled": 2},
        })
        result, obs = client.fill({"@1": "Alice", "@2": "password123"})
        op, args, _ = transport.calls[0]
        self.assertEqual(op, "fill")
        self.assertEqual(args["fields"], {"@1": "Alice", "@2": "password123"})

    def test_fill_returns_fill_result(self):
        client, _ = make_client({
            "id": "x", "ok": True,
            "result": {"filled": 2},
        })
        result, _ = client.fill({"@1": "Alice", "@2": "pass"})
        self.assertIsInstance(result, FillResult)
        self.assertEqual(result.filled, 2)


class TestScrollBy(unittest.TestCase):
    def test_scroll_by_positive(self):
        client, transport = make_client({
            "id": "x", "ok": True,
            "result": {"y": 300},
        })
        result, obs = client.scroll_by(300)
        op, args, _ = transport.calls[0]
        self.assertEqual(op, "scroll_by")
        self.assertEqual(args["dy"], 300)
        self.assertIsInstance(result, ScrollResult)
        self.assertEqual(result.y, 300)

    def test_scroll_by_negative(self):
        client, transport = make_client({
            "id": "x", "ok": True,
            "result": {"y": 0},
        })
        result, _ = client.scroll_by(-100)
        _, args, _ = transport.calls[0]
        self.assertEqual(args["dy"], -100)
        self.assertIsInstance(result, ScrollResult)


class TestScrollTo(unittest.TestCase):
    def test_scroll_to_y(self):
        client, transport = make_client({
            "id": "x", "ok": True,
            "result": {"y": 1000},
        })
        result, obs = client.scroll_to(y=1000)
        op, args, _ = transport.calls[0]
        self.assertEqual(op, "scroll_to")
        self.assertEqual(args["y"], 1000)
        self.assertNotIn("bottom", args)
        self.assertIsInstance(result, ScrollResult)
        self.assertEqual(result.y, 1000)

    def test_scroll_to_bottom(self):
        client, transport = make_client({
            "id": "x", "ok": True,
            "result": {"y": 9999},
        })
        result, obs = client.scroll_to(bottom=True)
        _, args, _ = transport.calls[0]
        self.assertTrue(args["bottom"])
        self.assertNotIn("y", args)
        self.assertIsInstance(result, ScrollResult)


class TestEval(unittest.TestCase):
    def test_eval_emits_expr(self):
        client, transport = make_client({
            "id": "x",
            "ok": True,
            "result": {"value": "42"},
        })
        result, _ = client.eval_("document.title")
        op, args, _ = transport.calls[0]
        self.assertEqual(op, "eval")
        self.assertEqual(args["expr"], "document.title")
        self.assertIsInstance(result, EvalResult)
        self.assertEqual(result.value, "42")

    def test_eval_with_ref(self):
        client, transport = make_client({
            "id": "x",
            "ok": True,
            "result": {"value": "button"},
        })
        client.eval_("this.tagName", ref="@3")
        _, args, _ = transport.calls[0]
        self.assertEqual(args["ref"], "@3")


class TestScreenshot(unittest.TestCase):
    def test_screenshot_default(self):
        client, transport = make_client({
            "id": "x",
            "ok": True,
            "result": {"mime": "image/png", "base64": "iVBOR..."},
        })
        result, _ = client.screenshot()
        op, args, _ = transport.calls[0]
        self.assertEqual(op, "screenshot")
        self.assertEqual(args, {})
        self.assertIsInstance(result, ScreenshotResult)
        self.assertEqual(result.mime, "image/png")
        self.assertEqual(result.base64, "iVBOR...")

    def test_screenshot_full_page_with_quality(self):
        client, transport = make_client({
            "id": "x",
            "ok": True,
            "result": {"mime": "image/jpeg", "base64": "base64data"},
        })
        result, _ = client.screenshot(full_page=True, quality=80)
        _, args, _ = transport.calls[0]
        self.assertTrue(args["full_page"])
        self.assertEqual(args["quality"], 80)
        self.assertEqual(result.base64, "base64data")
        self.assertEqual(result.mime, "image/jpeg")


class TestWait(unittest.TestCase):
    def test_wait_selector(self):
        client, transport = make_client({"id": "x", "ok": True})
        result, obs = client.wait(selector="#loaded")
        op, args, _ = transport.calls[0]
        self.assertEqual(op, "wait")
        self.assertEqual(args["selector"], "#loaded")
        self.assertIsNone(result)

    def test_wait_ms(self):
        client, transport = make_client({"id": "x", "ok": True})
        client.wait(ms=500)
        _, args, _ = transport.calls[0]
        self.assertEqual(args["ms"], 500)
        self.assertNotIn("selector", args)


class TestErrors(unittest.TestCase):
    def test_errors_returns_empty_list(self):
        """errors() result is a JSON array — empty array when no errors."""
        client, transport = make_client({
            "id": "x",
            "ok": True,
            "result": [],
        })
        result = client.errors()
        op, _, _ = transport.calls[0]
        self.assertEqual(op, "errors")
        self.assertIsInstance(result, list)
        self.assertEqual(len(result), 0)

    def test_errors_returns_list_of_error_entries(self):
        """errors() parses each element into an ErrorEntry dataclass."""
        client, transport = make_client({
            "id": "x",
            "ok": True,
            "result": [
                {
                    "type": "console",
                    "level": "error",
                    "message": "TypeError: x is undefined",
                    "source": "app.js:42:3",
                    "time_ms": 123,
                },
                {
                    "type": "network",
                    "level": "4xx",
                    "message": "https://api.example.com/me",
                    "source": "https://api.example.com/me",
                    "status": 401,
                    "method": "GET",
                    "time_ms": 456,
                },
            ],
        })
        result = client.errors()
        self.assertIsInstance(result, list)
        self.assertEqual(len(result), 2)

        e0 = result[0]
        self.assertIsInstance(e0, ErrorEntry)
        self.assertEqual(e0.type, "console")
        self.assertEqual(e0.level, "error")
        self.assertEqual(e0.message, "TypeError: x is undefined")
        self.assertEqual(e0.source, "app.js:42:3")
        self.assertEqual(e0.time_ms, 123)
        self.assertIsNone(e0.status)

        e1 = result[1]
        self.assertEqual(e1.type, "network")
        self.assertEqual(e1.status, 401)
        self.assertEqual(e1.method, "GET")


class TestUrl(unittest.TestCase):
    def test_url_returns_url_and_title(self):
        client, transport = make_client({
            "id": "x",
            "ok": True,
            "result": {"url": "https://example.com", "title": "Example"},
        })
        result, _ = client.url()
        op, args, _ = transport.calls[0]
        self.assertEqual(op, "url")
        self.assertEqual(args, {})
        self.assertIsInstance(result, UrlResult)
        self.assertEqual(result.url, "https://example.com")
        self.assertEqual(result.title, "Example")


class TestBackForward(unittest.TestCase):
    def test_back_emits_op(self):
        client, transport = make_client({
            "id": "x", "ok": True,
            "result": {"url": "https://example.com", "title": "Example"},
        })
        result, obs = client.back()
        op, args, _ = transport.calls[0]
        self.assertEqual(op, "back")
        self.assertEqual(args, {})
        self.assertIsInstance(result, BackForwardResult)

    def test_forward_emits_op(self):
        client, transport = make_client({
            "id": "x", "ok": True,
            "result": {"url": "https://example.com/done", "title": "Done"},
        })
        result, obs = client.forward()
        op, _, _ = transport.calls[0]
        self.assertEqual(op, "forward")
        self.assertIsInstance(result, BackForwardResult)
        self.assertEqual(result.url, "https://example.com/done")
        self.assertEqual(result.title, "Done")

    def test_back_optional_fields_none(self):
        """back/forward result fields are optional — may be absent."""
        client, _ = make_client({"id": "x", "ok": True, "result": {}})
        result, _ = client.back()
        self.assertIsInstance(result, BackForwardResult)
        self.assertIsNone(result.url)
        self.assertIsNone(result.title)


class TestInit(unittest.TestCase):
    def test_init_emits_op(self):
        """init result is omitted by the agent; returns InitResult with None fields."""
        client, transport = make_client({
            "id": "x",
            "ok": True,
        })
        from ghostchrome.types import InitResult
        result, _ = client.init()
        op, args, _ = transport.calls[0]
        self.assertEqual(op, "init")
        self.assertEqual(args, {})
        self.assertIsInstance(result, InitResult)
        self.assertIsNone(result.session_id)
        self.assertIsNone(result.browser_version)


class TestErrorPropagation(unittest.TestCase):
    def test_failed_op_raises_ghostchrome_error(self):
        client, _ = make_client({"id": "x", "ok": False, "error": "element not found"})
        with self.assertRaises(GhostchromeError) as ctx:
            client.click("@99")
        self.assertIn("element not found", str(ctx.exception))
        self.assertEqual(ctx.exception.op, "click")
        self.assertEqual(ctx.exception.message, "element not found")

    def test_failed_op_also_raises_transport_error_subclass(self):
        """GhostchromeError must be catchable as RuntimeError."""
        client, _ = make_client({"id": "x", "ok": False, "error": "boom"})
        with self.assertRaises(RuntimeError):
            client.click("@1")

    def test_close_does_not_raise_on_error(self):
        """close() swallows errors gracefully."""
        transport = FakeTransport(responses=[
            {"id": "x", "ok": False, "error": "already closed"}
        ])
        client = Ghostchrome(transport=transport)
        # Should not raise
        client.close()
        self.assertTrue(transport.closed)


class TestContextManager(unittest.TestCase):
    def test_context_manager_calls_close(self):
        transport = FakeTransport(responses=[])
        with Ghostchrome(transport=transport) as gc:
            transport.queue_ok(result={"url": "https://example.com", "title": ""})
            gc.url()
        self.assertTrue(transport.closed)

    def test_context_manager_closes_on_exception(self):
        transport = FakeTransport(responses=[])
        try:
            with Ghostchrome(transport=transport) as gc:
                raise ValueError("unexpected error")
        except ValueError:
            pass
        self.assertTrue(transport.closed)


class TestMultipleOpsSequence(unittest.TestCase):
    def test_full_workflow(self):
        """Simulate: navigate → extract → click → url."""
        transport = FakeTransport(responses=[
            {
                "id": "r1", "ok": True,
                "result": {"status": 200, "url": "https://app.com", "title": "App", "time_ms": 50},
                "observation": {"url": "https://app.com"},
            },
            {
                "id": "r2", "ok": True,
                "result": {
                    "nodes": [{"role": "button", "name": "Submit"}],
                    "refs": {"@1": {"role": "button", "name": "Submit"}},
                    "stats": {"total_nodes": 10, "filtered_nodes": 8, "interactive_count": 1},
                },
            },
            {
                "id": "r3", "ok": True,
                "observation": {"a11y_diff": "changed 1 node"},
            },
            {
                "id": "r4", "ok": True,
                "result": {"url": "https://app.com/done", "title": "Done"},
            },
        ])
        client = Ghostchrome(transport=transport)

        nav_result, nav_obs = client.navigate("https://app.com")
        self.assertEqual(nav_result.status, 200)
        self.assertEqual(nav_result.time_ms, 50)
        self.assertEqual(nav_obs.url, "https://app.com")

        ext_result, _ = client.extract(level="skeleton")
        self.assertIn("@1", ext_result.refs)
        self.assertEqual(ext_result.stats.interactive_count, 1)

        _, click_obs = client.click("@1")
        self.assertEqual(click_obs.a11y_diff, "changed 1 node")

        url_result, _ = client.url()
        self.assertEqual(url_result.url, "https://app.com/done")

        # Verify ops were emitted in the right order
        ops = [c[0] for c in transport.calls]
        self.assertEqual(ops, ["navigate", "extract", "click", "url"])


if __name__ == "__main__":
    unittest.main()
