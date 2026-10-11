#!/usr/bin/env python3
"""Run one read-only SELECT on the production database for diagnosis.

It goes through the same Portainer exec path the deploy script uses, so it
needs PORTAINER_URL, PORTAINER_API_KEY and ENDPOINT_ID in the environment
(load them the way deploy_local.sh does). The session is read-only and only a
single SELECT is accepted. Select the columns the diagnosis needs: no learner
transcripts, names or credentials. Never use this for tests or to change data.

Usage: set -a; source .env; source .env.deploy; set +a; python3 scripts/prod_query.py query.sql
"""
import json
import os
import ssl
import sys
import urllib.request

E = os.environ
BASE = E["PORTAINER_URL"].rstrip("/") + "/api/endpoints/" + E["ENDPOINT_ID"] + "/docker"
CTX = ssl.create_default_context(cafile="/etc/ssl/cert.pem") if os.path.exists("/etc/ssl/cert.pem") else ssl.create_default_context()


def api(method, path, data=None):
    headers = {"X-API-Key": E["PORTAINER_API_KEY"]}
    body = None
    if data is not None:
        body = json.dumps(data).encode()
        headers["Content-Type"] = "application/json"
    request = urllib.request.Request(BASE + path, data=body, method=method, headers=headers)
    with urllib.request.urlopen(request, timeout=60, context=CTX) as response:
        raw = response.read()
    try:
        return json.loads(raw) if raw else {}
    except ValueError:
        return raw


def main():
    query = open(sys.argv[1]).read().strip().rstrip(";")
    if not query.upper().startswith("SELECT") or ";" in query:
        raise SystemExit("only a single SELECT statement is accepted")
    containers = api("GET", "/containers/json?all=true")
    db = next(c for c in containers if "/toko-loop-db" in c["Names"] and c.get("Labels", {}).get("app") == "toko-loop")
    command = ["psql", "-U", "toko", "-d", "toko_loop", "-v", "ON_ERROR_STOP=1", "-qtAc", "SET default_transaction_read_only=on; " + query]
    exec_id = api("POST", "/containers/" + db["Id"] + "/exec", {"AttachStdout": True, "AttachStderr": True, "Cmd": command})["Id"]
    raw = api("POST", "/exec/" + exec_id + "/start", {"Detach": False, "Tty": False})
    chunks = []
    while isinstance(raw, bytes) and len(raw) >= 8:
        length = int.from_bytes(raw[4:8], "big")
        chunks.append(raw[8 : 8 + length])
        raw = raw[8 + length :]
    print(b"".join(chunks).decode())


if __name__ == "__main__":
    main()
