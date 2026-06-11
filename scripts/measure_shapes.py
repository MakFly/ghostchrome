"""Print the real result shape of each JSONL agent response read on stdin.

Used by scripts/measure-agent-ops.sh to keep the SDKs and docs honest:
the shapes printed here are the ground truth the binary actually emits.
"""
import json
import sys


def shape_of(r):
    if isinstance(r, dict):
        out = {}
        for k, v in r.items():
            out[k] = "str(%d)" % len(v) if k == "base64" else type(v).__name__
        return out
    if isinstance(r, list):
        head = {k: type(v).__name__ for k, v in r[0].items()} if r else "empty"
        return "array[%d] of %s" % (len(r), head)
    return r


for line in sys.stdin:
    line = line.strip()
    if not line or not line.startswith("{"):
        continue
    o = json.loads(line)
    result = o.get("result", "<<OMITTED>>")
    obs = "obs:" + ",".join(sorted(o["observation"])) if o.get("observation") else "obs:none"
    err = "  ERROR=%s" % o.get("error") if not o.get("ok") else ""
    print("%-11s ok=%s result=%s | %s%s" % (o.get("id"), o.get("ok"), shape_of(result), obs, err))
