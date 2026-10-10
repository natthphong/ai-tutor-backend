"""Run one non-interactive Codex call (gpt-6-luna, high effort) and return parsed JSON."""
import json, os, subprocess, tempfile

MODEL = "gpt-6-luna"
EFFORT = "high"
HERE = os.path.dirname(os.path.abspath(__file__))

def call(prompt: str, schema_path: str, timeout: int = 1500):
    # Codex runs read-only in an empty directory: it only writes text.
    os.makedirs(os.path.join(HERE, "tmp"), exist_ok=True)
    os.makedirs(os.path.join(HERE, "empty"), exist_ok=True)
    fd, out = tempfile.mkstemp(suffix=".json", dir=os.path.join(HERE, "tmp"))
    os.close(fd)
    cmd = ["codex", "exec", "-m", MODEL, "-c", f'model_reasoning_effort="{EFFORT}"',
           "--skip-git-repo-check", "--ephemeral", "-s", "read-only", "--color", "never",
           "-C", os.path.join(HERE, "empty"), "--output-schema", schema_path, "-o", out, "-"]
    try:
        p = subprocess.run(cmd, input=prompt, text=True, capture_output=True, timeout=timeout)
        raw = open(out, encoding="utf-8").read().strip()
        if p.returncode != 0 or not raw:
            raise RuntimeError(f"codex exit={p.returncode} stderr={p.stderr[-600:]} stdout={p.stdout[-300:]}")
        return json.loads(raw)
    finally:
        try: os.remove(out)
        except OSError: pass
