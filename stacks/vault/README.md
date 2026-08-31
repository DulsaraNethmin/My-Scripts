# vault

[HashiCorp Vault](https://developer.hashicorp.com/vault) in dev mode — secrets,
dynamic credentials, transit encryption — already initialised and unsealed,
with its built-in UI on the same port as the API.

```
spin up vault                              # http://localhost:8200
spin open vault                            # the UI, at /ui
spin cli vault -- status
spin cli vault -- kv put secret/hello foo=bar
spin cli vault -- kv get secret/hello
```

## Ports

| Service      | Host port | Container | Env var      |
|--------------|-----------|-----------|--------------|
| API and UI   | `8200`    | 8200      | `VAULT_PORT` |

## Credentials

| What       | Default             |
|------------|---------------------|
| Root token | `spinup-root-token` |

There is no username: you authenticate with a token. The UI's login screen
takes it in the "Token" method, and the CLI reads it from `VAULT_TOKEN`.

```
export VAULT_ADDR=http://localhost:8200
export VAULT_TOKEN=spinup-root-token
vault kv get secret/hello
```

In dev mode the root token is normally generated and printed to the logs;
`VAULT_TOKEN` here fixes it instead, so scripts do not have to scrape it back
out of `spin logs vault`.

## Dev mode, and what it costs

This stack runs `vault server -dev`, which is what makes it a one-command
service: a real Vault starts *sealed*, and you would have to initialise it,
save the unseal keys and unseal it by hand on every start.

What you give up:

- **Nothing is persisted.** The storage is in memory. `spin restart vault`
  is an empty Vault, and the stack declares no volumes because there is
  nothing to keep.
- A KV v2 secrets engine is pre-mounted at `secret/`, which a real Vault has
  to be told to mount.
- The root token has every capability, so policies never get exercised.
- TLS is off. The API is plain HTTP.

For learning the API, wiring an app against a real Vault client or testing a
policy's happy path, that is exactly the trade you want. Nothing here should
inform how a production Vault is configured.

## Notes

- The UI is served by Vault itself, not a separate container, so this stack
  has no `gui` profile and `--gui` has nothing to select.
- `VAULT_ADDR` and `VAULT_TOKEN` are set inside the container, which is what
  makes `spin cli vault` already pointed at the server and authenticated —
  `spin cli vault -- <any vault subcommand>`.
- No `IPC_LOCK` capability: it exists so Vault can `mlock` its memory against
  being swapped to disk, and dev mode turns mlock off.
- The healthcheck reads `/v1/sys/health`, which answers 200 only when Vault is
  initialised, unsealed and active — the three things that can be wrong with
  one.
- `hashicorp/vault` is BUSL-licensed. That is fine for local use; if a licence
  matters to you, [OpenBao](https://openbao.org/) is the OSS fork of the last
  MPL release.
