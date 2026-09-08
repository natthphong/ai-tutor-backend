# Local testing

Run all backend checks with one command:

```sh
bash scripts/validate.sh
```

## บัญชี QA สำหรับ local test

```text
username: qa_learner
password: qa-password-only
```

บัญชีนี้ใช้กับฐานข้อมูล local/test ที่ทิ้งได้เท่านั้น ห้ามใช้หรือ commit credential production

The default is the local Docker database at `localhost:55432/toko_loop_test`, using user `toko`. It is a dedicated disposable database. Integration tests truncate only the database supplied as `TEST_DATABASE_URL`. The validator parses the URL path and requires the database name to end in `_test`; it also requires a loopback host unless `QA_ALLOW_REMOTE_TEST_DB=1` is explicitly set for a dedicated remote test database.

To use another isolated database:

```sh
TEST_DATABASE_URL='postgres://toko:toko-local-only@localhost:55432/toko_loop_test?sslmode=disable' bash scripts/validate.sh
```

No test calls Gemini. Backend integration tests use a local fake provider, while unit tests use fixtures and fakes.

## Learn Ebook fixtures

The supplied book is imported locally into ignored `.ebook/`; it is never committed or copied into test output. `internal/ebook/book_test.go` builds only synthetic manifests and packs. It verifies the expected 392-page/145-unit manifest shape, source-derived item-ID coverage, answer sanitization for learners, and rejection of omitted, invented, or repeated question IDs. Run the local importer only with the private source file:

```sh
python3 -m venv .venv-ebook
. .venv-ebook/bin/activate
pip install -r scripts/requirements-ebook.txt
# Install Poppler (pdftotext/pdftoppm) using your OS package manager first.
python3 scripts/import_ebook.py /private/path/book.pdf --output .ebook
```

Set `EBOOK_DIR` only when the private imported directory is available. The normal validator does not require the book or make any model call.

For browser/API work against a local server, `python3 scripts/qa_local.py` creates or reuses the local-only fixture account above. It writes ignored `.toko-qa.json`; never use that password outside a local disposable database. Production release QA credentials stay in ignored environment files and are never documented or committed.

The frontend companion command is `npm run validate` in `../frontend`; it typechecks, runs fixture-based UI/API/audio tests, and builds without using a backend or an AI provider.

## Extending Ebook coverage

Keep source fixtures synthetic. Add new importer cases in `internal/ebook/book_test.go`; add API cases in `internal/app/ebook_feature_test.go` for owner isolation, request-id retries, answer redaction, per-user progress, and the two independent speaking rounds. Tests that need a real imported book must use a private `EBOOK_DIR` outside Git and must not copy page images, source text, or answer keys into reports or snapshots.
