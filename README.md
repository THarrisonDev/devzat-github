# devzat-github

A [Devzat](https://github.com/quackduck/devzat) plugin (Go) that lets anyone in
chat pull up GitHub issues and pull requests with a `gh` command.

```
gh issue <owner/repo> <number>   Show an issue
gh pr    <owner/repo> <number>   Show a pull request
gh issues <owner/repo>           List open issues
gh prs   <owner/repo>            List open pull requests
gh help                          Show this help
```

`gh issue` rejects a number that is actually a pull request (and points you at
`gh pr`), matching `gh` CLI behavior. `gh issues` likewise lists issues only.

It talks to Devzat's gRPC plugin API using the maintained
[`devzatapi`](https://github.com/quackduck/devzat/tree/main/devzatapi) client, so
it ships as a single static binary.

## Build

```bash
go build -o devzat-github .
```

## Configure

| Env | Required | Purpose |
|---|---|---|
| `DEVZAT_ADDRESS` | yes\* | Plugin-API address, e.g. `localhost:5556` |
| `DEVZAT_TOKEN` | yes | Plugin-API token from `grant` in Devzat |
| `GITHUB_TOKEN` | no | Raises rate limit (60/hr → 5000/hr) and enables private repos |
| `GITHUB_DEFAULT_REPO` | no | `owner/repo`, so `gh pr 123` works without it |

\* May instead be passed as the first CLI argument, which takes precedence.

## Run

```bash
DEVZAT_TOKEN=... ./devzat-github localhost:5556
```

Try a lookup without a server (prints the formatted message):

```bash
./devzat-github --print pr quackduck/devzat 5
```

## Server setup (once)

The `devzat-deploy` folder already wires this up: its `devzat.yml` sets
`integration_config: /opt/devzat/devzat-integrations.yml` and
`install.sh` installs that file (`rpc: { port: 5556 }`). Re-run
`sudo ./install.sh` there, or apply the two edits by hand, then
`systemctl restart devzat`.

Port 5556 is internal only — Devzat's RPC listener is not opened by the deploy
firewall (which allows just 22/2221/443). Do not expose it publicly.

## Install (systemd)

From this directory on the Droplet:

```bash
sudo DEVZAT_TOKEN=dvz@... GITHUB_TOKEN=ghp_... ./install.sh
```

The script builds the binary, installs `/usr/local/bin/devzat-github`, drops in
`devzat-github.service`, and starts it. Tokens may be passed inline (as above) or
left blank and edited into `/etc/devzat-github.env` (mode `600`) afterwards. An
existing env file is preserved; only the values you pass are updated.

Mint the `DEVZAT_TOKEN` first, as an admin in chat:

```
grant <you> github-bot
```

## Operate

```bash
systemctl status devzat-github
journalctl -u devzat-github -f
systemctl restart devzat-github
```
