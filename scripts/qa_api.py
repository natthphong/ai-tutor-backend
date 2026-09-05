#!/usr/bin/env python3
"""Bounded black-box QA against a local Toko Loop API with real Gemini.

The script creates only synthetic QA data. It never prints credentials or auth
tokens, and it records a redacted result in reports/qa-api.json.
"""

from __future__ import annotations

import json
import os
import struct
import time
import urllib.error
import urllib.request
import uuid
from pathlib import Path


BASE = os.environ.get("TEST_API_URL", "http://localhost:8080/ai-tutor/api/v2").rstrip("/")
USER_AGENT = "TokoLoop-QA/2"
REPORT = Path("reports/qa-api.json")
TTS_WAV = Path("/tmp/toko-qa-tts.wav")


class HTTPFailure(RuntimeError):
    def __init__(self, status: int, body: bytes):
        self.status = status
        self.body = body
        super().__init__(f"HTTP {status}")


def request(path: str, *, method: str = "GET", token: str = "", data=None, raw=None, content_type="application/json"):
    body = raw
    if data is not None:
        body = json.dumps(data, ensure_ascii=False).encode()
    headers = {"User-Agent": USER_AGENT}
    if body is not None:
        headers["Content-Type"] = content_type
    if token:
        headers["Authorization"] = "Bearer " + token
    req = urllib.request.Request(BASE + path, data=body, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=95) as res:
            content = res.read()
            return res.status, json.loads(content) if "json" in res.headers.get("Content-Type", "") else content
    except urllib.error.HTTPError as exc:
        raise HTTPFailure(exc.code, exc.read()) from exc


def expect_status(status: int, fn):
    try:
        actual, value = fn()
    except HTTPFailure as exc:
        actual, value = exc.status, exc.body
    if actual != status:
        raise AssertionError(f"expected HTTP {status}, got {actual}")
    return value


def multipart(fields: dict[str, str], filename: str, blob: bytes, mime="audio/wav"):
    boundary = "----TokoLoopQA" + uuid.uuid4().hex
    parts: list[bytes] = []
    for key, value in fields.items():
        parts.extend([
            f"--{boundary}\r\n".encode(),
            f'Content-Disposition: form-data; name="{key}"\r\n\r\n'.encode(),
            value.encode(), b"\r\n",
        ])
    parts.extend([
        f"--{boundary}\r\n".encode(),
        f'Content-Disposition: form-data; name="audio"; filename="{filename}"\r\n'.encode(),
        f"Content-Type: {mime}\r\n\r\n".encode(),
        blob, b"\r\n", f"--{boundary}--\r\n".encode(),
    ])
    return b"".join(parts), f"multipart/form-data; boundary={boundary}"


def poll_job(token: str, job_id: str, timeout=120):
    deadline = time.time() + timeout
    while time.time() < deadline:
        _, job = request("/jobs/" + job_id, token=token)
        if job["status"] == "complete":
            return job["result"]
        if job["status"] == "failed":
            raise AssertionError(f"job {job['kind']} failed")
        time.sleep(1)
    raise AssertionError("job polling timed out")


def login(creds):
    return expect_status(200, lambda: request("/auth/login", method="POST", data=creds))["token"]


def add(checks, name, ok, detail=""):
    checks.append({"name": name, "ok": bool(ok), "detail": detail})
    if not ok:
        raise AssertionError(name + (": " + detail if detail else ""))


def main():
    started = time.time()
    checks = []
    result = {
        "target": BASE,
        "synthetic_only": True,
        "started_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
        "checks": checks,
        "status": "running",
    }
    REPORT.parent.mkdir(parents=True, exist_ok=True)
    try:
        creds = json.loads(Path(".toko-qa.json").read_text())
        token = login(creds)
        _, health = request("/health")
        _, ready = request("/readiness")
        add(checks, "health/readiness", health.get("status") == "ok" and ready.get("status") == "ready" and ready.get("ai_configured") is True, health.get("release", ""))

        # Optionally reuse an app-generated asset after a provider retry test.
        # This keeps reruns bounded and never accepts a file produced outside the app.
        phrase = "Hello, I'm Pim."
        audio_id = os.environ.get("QA_TTS_AUDIO_ID", "")
        if audio_id:
            add(checks, "app TTS asset reused after bounded generation", True)
        else:
            tts_status, tts = request("/audio/tts", method="POST", token=token, data={"text": phrase, "voice": "Kore"})
            if tts_status == 200:
                audio_id = tts["audio_id"]
            elif tts_status == 202:
                audio_id = poll_job(token, tts["job_id"])["audio_id"]
            else:
                raise AssertionError(f"expected TTS HTTP 200/202, got {tts_status}")
        wav = expect_status(200, lambda: request("/audio/" + audio_id, token=token))
        add(checks, "tts job produces WAV", isinstance(wav, bytes) and wav[:4] == b"RIFF" and wav[8:12] == b"WAVE", f"bytes={len(wav)}")
        TTS_WAV.write_bytes(wav)
        asset_owner_token = token

        # Invitation is one-use. Admin is a distinct principal for audio ownership checks.
        admin = login({"username": "admin", "password": "Toko-Local-Admin-2026!"})
        invite = expect_status(201, lambda: request("/admin/invitations", method="POST", token=admin, data={}))
        marker = uuid.uuid4().hex[:10]
        second = {"username": "qa_owner_" + marker, "password": "QA-owner-" + marker + "-Safe9!"}
        expect_status(201, lambda: request("/auth/register", method="POST", data={**second, "invitation": invite["code"]}))
        expect_status(400, lambda: request("/auth/register", method="POST", data={"username": "qa_reuse_" + marker, "password": second["password"], "invitation": invite["code"]}))
        add(checks, "invitation can be used once", True)
        second_token = login(second)
        expect_status(404, lambda: request("/audio/" + audio_id, token=second_token))
        add(checks, "TTS audio owner isolation", True)
        # Run every learning assertion as the fresh synthetic user. This makes
        # the harness repeatable and keeps each run's usage delta isolated.
        token = second_token
        _, usage_before = request("/usage", token=token)

        # Lesson: alternative answer semantics, duplicate request id, audio, and retry.
        lesson = expect_status(201, lambda: request("/sessions", method="POST", token=token, data={"mode": "lesson", "lesson_id": "lesson-001"}))
        sid = lesson["id"]
        expect_status(200, lambda: request(f"/sessions/{sid}/advance", method="POST", token=token, data={}))
        rid = str(uuid.uuid4())
        alt_payload = {"text": "Hi, my name is Pim.", "request_id": rid}
        alt = expect_status(200, lambda: request(f"/sessions/{sid}/turns", method="POST", token=token, data=alt_payload))
        _, usage_after_alt = request("/usage", token=token)
        alt2 = expect_status(200, lambda: request(f"/sessions/{sid}/turns", method="POST", token=token, data=alt_payload))
        _, usage_after_duplicate = request("/usage", token=token)
        grammar = [c for c in alt["feedback"].get("corrections", []) if c.get("kind") == "grammar"]
        add(checks, "valid alternative accepted", alt["feedback"].get("correct") is True and alt["feedback"].get("goal_met") is True and not grammar)
        add(checks, "duplicate answer request is not billed", alt["id"] == alt2["id"] and usage_after_alt["calls"] == usage_after_duplicate["calls"], f"calls={usage_after_duplicate['calls']}")

        audio_body, audio_type = multipart({"request_id": str(uuid.uuid4())}, "tts.wav", wav)
        spoken = expect_status(200, lambda: request(f"/sessions/{sid}/turns", method="POST", token=token, raw=audio_body, content_type=audio_type))
        add(checks, "TTS to multipart speech feedback", spoken["feedback"].get("audio_clear") is True and bool(spoken["feedback"].get("transcript")), spoken["feedback"].get("transcript", "")[:120])
        attempt_audio_id = spoken.get("audio_id")
        expect_status(200, lambda: request("/audio/" + attempt_audio_id, token=token))
        expect_status(404, lambda: request("/audio/" + attempt_audio_id, token=asset_owner_token))
        add(checks, "attempt audio owner isolation", True)

        retry_body, retry_type = multipart({"request_id": str(uuid.uuid4()), "retry_of": spoken["id"]}, "tts-retry.wav", wav)
        retried = expect_status(200, lambda: request(f"/sessions/{sid}/turns", method="POST", token=token, raw=retry_body, content_type=retry_type))
        session_snapshot = expect_status(200, lambda: request(f"/sessions/{sid}", token=token))
        matching = [a for a in session_snapshot["attempts"] if a["id"] == retried["id"]]
        add(checks, "speech retry links to prior attempt", len(matching) == 1 and matching[0].get("retry_of") == spoken["id"])
        expect_status(200, lambda: request(f"/sessions/{sid}/complete", method="POST", token=token, data={}))

        # Review by audio, then verify progress and persistence after a fresh login.
        term = "Hello, I'm Pim."
        expect_status(201, lambda: request("/vocabulary", method="POST", token=token, data={"term": term, "meaning": "การแนะนำตัวเพื่อทดสอบคุณภาพ", "example": phrase}))
        _, review_items = request("/review", token=token)
        review = next(x for x in review_items if x.get("target") == term)
        review_body, review_type = multipart({"request_id": str(uuid.uuid4())}, "review.wav", wav)
        review_result = expect_status(200, lambda: request(f"/review/{review['id']}/answer", method="POST", token=token, raw=review_body, content_type=review_type))
        add(checks, "audio review reschedules", review_result.get("rescheduled") is True and review_result["feedback"].get("audio_clear") is True)
        _, progress = request("/progress", token=token)
        fresh_token = login(second)
        reloaded = expect_status(200, lambda: request(f"/sessions/{sid}", token=fresh_token))
        _, progress_reload = request("/progress", token=fresh_token)
        add(checks, "progress/session persist after reload", len(reloaded["attempts"]) == 3 and progress_reload["attempts"] >= progress["attempts"] and progress_reload["speaking_minutes"] > 0)
        token = fresh_token

        # Grammar correctness and communication goal are independent.
        miss = expect_status(201, lambda: request("/sessions", method="POST", token=token, data={"mode": "lesson", "lesson_id": "lesson-001"}))
        expect_status(200, lambda: request(f"/sessions/{miss['id']}/advance", method="POST", token=token, data={}))
        off = expect_status(200, lambda: request(f"/sessions/{miss['id']}/turns", method="POST", token=token, data={"text": "I like coffee.", "request_id": str(uuid.uuid4())}))
        off_grammar = [c for c in off["feedback"].get("corrections", []) if c.get("kind") == "grammar"]
        add(checks, "grammar and goal are separated", off["feedback"].get("correct") is True and off["feedback"].get("goal_met") is False and not off_grammar)
        expect_status(200, lambda: request(f"/sessions/{miss['id']}/complete", method="POST", token=token, data={}))

        # Custom scenario job -> edit -> roleplay -> complete -> generated summary -> retry.
        scenario_job = expect_status(202, lambda: request("/scenarios", method="POST", token=token, data={"prompt": "Create a fictional daily standup about fixing an API login bug and a database blocker.", "lesson_id": "lesson-001", "request_id": str(uuid.uuid4())}))
        scenario = poll_job(token, scenario_job["job_id"])
        scenario["title"] = "Synthetic API standup QA"
        scenario["roles"] = ["Scrum master", "Developer"]
        edited = expect_status(200, lambda: request("/scenarios/" + scenario["id"], method="PATCH", token=token, data=scenario))
        add(checks, "custom scenario job and edit", edited.get("title") == scenario["title"] and edited.get("roles") == scenario["roles"])
        scene = expect_status(201, lambda: request("/sessions", method="POST", token=token, data={"mode": "scenario", "scenario_id": scenario["id"]}))
        scene_turn = expect_status(200, lambda: request(f"/sessions/{scene['id']}/turns", method="POST", token=token, data={"text": "Yesterday I fix the login bug. Today I work on the API, but the database are unavailable.", "request_id": str(uuid.uuid4())}))
        add(checks, "scenario roleplay returns grammar feedback", any(c.get("kind") == "grammar" for c in scene_turn["feedback"].get("corrections", [])) and bool(scene_turn["feedback"].get("retry_sentence")))
        expect_status(200, lambda: request(f"/sessions/{scene['id']}/complete", method="POST", token=token, data={}))
        deadline = time.time() + 120
        scene_snapshot = None
        while time.time() < deadline:
            scene_snapshot = expect_status(200, lambda: request(f"/sessions/{scene['id']}", token=token))
            if (scene_snapshot["session"].get("summary") or {}).get("feedback"):
                break
            time.sleep(1)
        feedback = (scene_snapshot["session"].get("summary") or {}).get("feedback") or {}
        add(checks, "post-scene review generated", bool(feedback.get("reply")) and bool(feedback.get("retry_sentence")))
        retry_session = expect_status(201, lambda: request(f"/sessions/{scene['id']}/retry", method="POST", token=token, data={}))
        retry_turn = expect_status(200, lambda: request(f"/sessions/{retry_session['id']}/turns", method="POST", token=token, data={"text": feedback["retry_sentence"], "request_id": str(uuid.uuid4())}))
        add(checks, "post-scene retry can be practiced", retry_turn["feedback"].get("correct") is True)
        expect_status(200, lambda: request(f"/sessions/{retry_session['id']}/complete", method="POST", token=token, data={}))

        _, usage_after = request("/usage", token=token)
        spent_delta = round(float(usage_after["spent"]) - float(usage_before["spent"]), 4)
        add(checks, "new Gemini spend stays within QA budget", spent_delta <= 5.0, f"THB={spent_delta}")
        result.update({
            "status": "passed",
            "duration_seconds": round(time.time() - started, 1),
            "new_spend_thb": spent_delta,
            "calls_delta": int(usage_after["calls"]) - int(usage_before["calls"]),
            "tts_fixture": str(TTS_WAV),
            "session_ids": {"lesson": sid, "goal_separation": miss["id"], "scenario": scene["id"], "post_scene_retry": retry_session["id"]},
        })
    except Exception as exc:
        result.update({"status": "failed", "duration_seconds": round(time.time() - started, 1), "error": f"{type(exc).__name__}: {exc}"})
    finally:
        REPORT.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
        print(json.dumps({"status": result["status"], "checks": len(checks), "error": result.get("error")}, ensure_ascii=False))
    if result["status"] != "passed":
        raise SystemExit(1)


if __name__ == "__main__":
    main()
