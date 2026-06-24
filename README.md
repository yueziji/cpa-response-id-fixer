# CPA Response ID Fixer

Small CLIProxyAPI native plugin that repairs malformed OpenAI Responses stream chunks.

It registers `response_stream_interceptor` and only changes chunks whose JSON payload is:

```json
{"type":"response.completed","response":{...}}
```

If `response.id` is missing or empty, the plugin:

1. reuses a previous response id from recent stream history, such as `response.created.response.id`;
2. otherwise creates a deterministic fallback id with the `resp_cpa_` prefix.

All other chunks pass through unchanged.

## Performance notes

CPA calls enabled stream interceptors for stream chunks before sending them downstream. This plugin now exits early unless `SourceFormat` is `openai-response`, so non-Responses streams avoid the heavier body/history decoding inside the plugin.

There is still a small global interceptor-call cost while the plugin is enabled, including one stream header initialization call before the first payload. For zero overhead on non-Responses streams, this repair would need to live in CPA core or CPA would need protocol-scoped stream interceptors.

## Build

From this directory:

```powershell
New-Item -ItemType Directory -Path '.\bin'
go test ./...
$env:CGO_ENABLED = '1'
go build -buildmode=c-shared -o '.\bin\cpa-response-id-fixer.dll' .
```

`go build -buildmode=c-shared` also emits a `.h` file next to the DLL. CPA only needs the `.dll`.
On Windows this build requires a C compiler such as MinGW-w64 `gcc` or LLVM/Clang in `PATH`.

## GitHub release

This repository includes a GitHub Actions workflow at `.github/workflows/release.yml`.

It builds:

- `cpa-response-id-fixer-windows-amd64.zip`
- `cpa-response-id-fixer-linux-amd64.zip`

Each zip contains a stable plugin file name:

- Windows: `cpa-response-id-fixer.dll`
- Linux: `cpa-response-id-fixer.so`

Push a version tag to publish a Release automatically:

```powershell
git tag v0.1.0
git push origin v0.1.0
```

You can also run the workflow manually from GitHub Actions and provide a tag such as `v0.1.0`.

## CPA config

Either place the DLL under CPA's configured plugin directory, or point `plugins.path` at the absolute DLL path if your CPA version/config uses that form.

Minimal config when the DLL file name is `cpa-response-id-fixer.dll`:

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    cpa-response-id-fixer:
      enabled: true
      priority: 100
```

Use a high priority if you want this repair to run before other stream interceptors.
