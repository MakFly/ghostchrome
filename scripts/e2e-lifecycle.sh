#!/usr/bin/env bash
# e2e lifecycle/leak test for ghostchrome.
# Runs a command matrix and asserts: no command errors, no leftover
# ghostchrome browser/serve processes, sessions reuse (no pile-up), and
# serve self-exits when its Chrome dies.
#
# Must run in a SINGLE shell so detached session serves persist across steps.
# Usage: GC=./ghostchrome bash scripts/e2e-lifecycle.sh
set -u

GC="${GC:-ghostchrome}"
SITE_LIGHT="https://example.com"
SITE_HEAVY="${SITE_HEAVY:-https://iautos.fr}"
FAILS=0
pass() { echo "  PASS: $1"; }
fail() { echo "  FAIL: $1"; FAILS=$((FAILS+1)); }

# ghostchrome-attributable live processes (serve/agent + chrome on a gc profile)
gc_procs() { ps -eo args 2>/dev/null | grep -v grep | grep -E "ghostchrome (serve|agent)|user-data-dir=[^ ]*\.ghostchrome/profiles" ; }
gc_count() { gc_procs | grep -c . ; }
serve_count() { ps -eo args 2>/dev/null | grep -v grep | grep -c "ghostchrome serve" ; }

echo "== ghostchrome e2e lifecycle =="
"$GC" sessions kill-all >/dev/null 2>&1
sleep 1
BASE=$(gc_count)
echo "baseline gc procs: $BASE"

# 1) auto-launch must leave nothing behind
OUT=$("$GC" preview "$SITE_LIGHT" 2>&1); RC=$?
sleep 1
[ $RC -eq 0 ] && pass "auto-launch preview exit 0" || fail "auto-launch preview exit $RC"
echo "$OUT" | grep -qi "^error:" && fail "auto-launch printed an error" || pass "auto-launch no error line"
[ "$(gc_count)" -eq "$BASE" ] && pass "auto-launch left no process" || fail "auto-launch leaked $(($(gc_count)-BASE)) proc(s)"

# 2) session spawn + reuse (serve stays at 1)
"$GC" -s e2e1 goto "$SITE_LIGHT" >/dev/null 2>&1; RC=$?
sleep 1
[ $RC -eq 0 ] && pass "session spawn exit 0" || fail "session spawn exit $RC"
S1=$(serve_count)
"$GC" -s e2e1 reload >/dev/null 2>&1
"$GC" -s e2e1 url  >/dev/null 2>&1 || true
sleep 1
S2=$(serve_count)
[ "$S2" -eq "$S1" ] && pass "session reuse did not pile up serves ($S2)" || fail "serves piled up: $S1 -> $S2"

# 3) heavy site + stealth: the original 'stealth: context canceled' bug.
# The point of this case is that stealth never ABORTS the command; the heavy
# site is genuinely slow (~25s), so give it budget and treat a pure
# network/timeout outcome as a skip (not a stealth/lifecycle failure).
HOUT=$("$GC" --stealth --timeout 60 -s e2e2 preview "$SITE_HEAVY" 2>&1); RC=$?
if echo "$HOUT" | grep -qi "error: stealth"; then
  fail "stealth aborted the command (the bug)"
elif echo "$HOUT" | grep -qi "\[200\]"; then
  pass "stealth did not abort + heavy site loaded (200)"
elif echo "$HOUT" | grep -qiE "deadline|timeout|dial|could not|net::"; then
  echo "  SKIP: heavy site $SITE_HEAVY slow/unreachable today (rc=$RC)"
else
  fail "heavy-site preview failed unexpectedly (rc=$RC): $(echo "$HOUT" | head -1)"
fi

# 4) two sessions -> kill-all returns to baseline
"$GC" -s e2e3 goto "$SITE_LIGHT" >/dev/null 2>&1
sleep 1
echo "  (serves with 3 sessions: $(serve_count))"
"$GC" sessions kill-all >/dev/null 2>&1
sleep 2
[ "$(gc_count)" -eq "$BASE" ] && pass "kill-all returned to baseline ($BASE)" || fail "kill-all left $(($(gc_count)-BASE)) proc(s)"

# 5) serve self-exits when its Chrome dies (no orphan serve)
"$GC" -s e2edead goto "$SITE_LIGHT" >/dev/null 2>&1
sleep 1
BEFORE=$(serve_count)
# kill only the Chrome of this session (by its profile dir), leave serve alone
pkill -f "user-data-dir=[^ ]*\.ghostchrome/profiles/e2edead" 2>/dev/null
echo "  killed e2edead Chrome; waiting for serve to notice..."
sleep 6
AFTER=$(serve_count)
[ "$AFTER" -lt "$BEFORE" ] && pass "serve self-exited after Chrome died ($BEFORE -> $AFTER)" || fail "orphan serve survived Chrome death ($BEFORE -> $AFTER)"
"$GC" sessions prune >/dev/null 2>&1
"$GC" sessions kill-all >/dev/null 2>&1
sleep 1

# 6) final: nothing left
FINAL=$(gc_count)
[ "$FINAL" -eq "$BASE" ] && pass "final state clean ($FINAL == baseline)" || fail "final leaked: $FINAL vs baseline $BASE"
gc_procs | sed 's/^/    leftover: /' | cut -c1-100

echo ""
if [ "$FAILS" -eq 0 ]; then echo "== E2E PASS =="; exit 0; else echo "== E2E FAIL ($FAILS) =="; exit 1; fi
