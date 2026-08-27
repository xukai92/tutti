# Running `tuttid` on a remote machine

By default the desktop app spawns and supervises a local `tuttid` child process
and talks to it over `127.0.0.1`. **Remote daemon mode** lets the desktop client
instead connect to a `tuttid` running on another machine, so that everything the
daemon owns — agent sessions, terminals, files, browser/computer use, connectors
— runs remotely.

## How it works

The client talks to `tuttid` entirely over HTTP + WebSocket with a bearer token.
There is no other coupling: the desktop's transport already routes every request
through a single resolved endpoint (`resolveDesktopDaemonEndpoint`), and the WS
URL builder derives `ws`/`wss` from that endpoint's scheme. Remote mode simply
points that endpoint at a remote base URL and supplies the shared token, and
skips launching a local daemon (`RemoteTuttid` in `daemon/tuttidManager.ts`
supervises nothing and only waits for the remote to report healthy).

## Server side (remote machine)

Build and run the daemon bound to a reachable address with a shared secret:

```sh
pnpm build:go   # or: cd services/tuttid && go build -o tuttid .

TUTTID_ADDR=127.0.0.1:4545 \
TUTTID_ACCESS_TOKEN="$(openssl rand -base64 32)" \
./tuttid
```

`TUTTID_ADDR` defaults to `127.0.0.1:4545`. `TUTTID_ACCESS_TOKEN` is required.

> **Security.** The access token is sent as an HTTP bearer header and, for
> WebSocket streams, as an `access_token` query parameter. Never expose the
> daemon on a raw `0.0.0.0` port over an untrusted network. Terminate TLS in
> front of it (reverse proxy) or reach it through an SSH tunnel, and prefer
> binding to loopback + tunnelling over binding to a public interface.

### Recommended: SSH tunnel

Keep the daemon on loopback and forward a local port:

```sh
ssh -N -L 4545:127.0.0.1:4545 user@remote-host
```

The client then connects to `http://127.0.0.1:4545` while the daemon runs
remotely.

## Client side (desktop)

Set two environment variables before launching the desktop app:

| Variable | Meaning |
| --- | --- |
| `TUTTID_REMOTE_URL` | Base URL of the remote daemon. A bare `host:port` defaults to `https`. |
| `TUTTID_REMOTE_ACCESS_TOKEN` | Must match the remote daemon's `TUTTID_ACCESS_TOKEN`. |

Examples:

```sh
# Through an SSH tunnel (loopback):
TUTTID_REMOTE_URL="http://127.0.0.1:4545" \
TUTTID_REMOTE_ACCESS_TOKEN="<same token as the daemon>" \
pnpm dev:desktop

# Direct, over TLS:
TUTTID_REMOTE_URL="https://tutti.example.com" \
TUTTID_REMOTE_ACCESS_TOKEN="<same token as the daemon>" \
pnpm dev:desktop
```

When `TUTTID_REMOTE_URL` is set the desktop does **not** start a local daemon.
Startup blocks until the remote reports healthy, so a wrong URL/token surfaces
immediately rather than as a later connection failure.

## Caveats

- **Everything is remote.** Files, terminals, and agent working directories live
  on the daemon's machine, not the client's. This is the intended behaviour of
  remote mode, but it means local files are not visible unless separately synced
  or mounted on the remote host.
- **Single client assumption.** The daemon was designed to be supervised by one
  desktop instance. Pointing multiple clients at one daemon is not yet a
  supported configuration.
- **CORS / origin.** The daemon applies CORS and origin checks; a non-loopback
  client origin may require adjustment.
