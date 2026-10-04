# Catalift API

The contract between the web app and the Go API (ADR-0007). The spec comes before the handler: a route is added to `api/openapi.yaml` in the same merge request as its code, and the web app uses only the client generated from it (tenet 7).

- Spec: `api/openapi.yaml` (OpenAPI 3.1, 23 paths, 26 operations).
- Readable design: [API.md](API.md), generated from the spec; never edit it by hand.
- Docs route: unconfirmed: no docs route. Add `GET /docs` on the API serving Scalar or Redoc over `api/openapi.yaml`, behind the session like every other route.
- Style: the bearing `openapi-spec` style rules, plus this project's conventions in `info.x-conventions` (session cookie and CSRF instead of bearer tokens, `/v1` base path, string ids, `cost_micro_usd`, multipart uploads, version-checked edits, rate-limit numbers).

## Lint

```bash
npx @redocly/cli@2.54.3 lint api/openapi.yaml
```

## Regenerate API.md

```bash
uv run --quiet --with pyyaml==6.0.3 python "$BEARING/skills/openapi-spec/scripts/api_doc.py" --spec api/openapi.yaml --style "$BEARING/skills/openapi-spec/references/api-style.md" --out docs/api/API.md
```

`$BEARING` is the bearing plugin directory on your machine.

## Conformance

When the repository exists, `make api-conformance` (from the bearing `openapi-spec` template) runs Schemathesis against a local or qa server at `API_BASE_URL`, and a CI job of the same name runs it in the `test` stage after `docker compose up -d --wait` and `make migrate`. It is not added yet because there is no repository or Makefile.

## Breaking changes

Once the spec is in git and tagged, `oasdiff breaking <last tag spec> api/openapi.yaml` is the gate; a removal is `deprecated: true` plus a `Sunset` header for at least 90 days.
