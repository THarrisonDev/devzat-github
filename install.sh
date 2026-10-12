#!/usr/bin/env bash
# Build and install the Devzat GitHub plugin.
#
# Run as root from this directory:
#   sudo ./install.sh
#
# Pass tokens inline to avoid editing the env file afterwards:
#   sudo DEVZAT_TOKEN=dvz@... GITHUB_TOKEN=ghp_... ./install.sh
set -euo pipefail

SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN=/usr/local/bin/devzat-github
SVC=/etc/systemd/system/devzat-github.service
ENV_FILE=/etc/devzat-github.env

echo "==> Building devzat-github"
( cd "$SRC_DIR" && go build -o "$BIN" . )
chmod 755 "$BIN"

echo "==> Installing systemd unit"
install -m 0644 "$SRC_DIR/devzat-github.service" "$SVC"

if [ ! -f "$ENV_FILE" ]; then
    echo "==> Creating $ENV_FILE"
    install -m 0600 /dev/null "$ENV_FILE"
    cat >"$ENV_FILE" <<'EOF'
# Devzat GitHub plugin configuration
# DEVZAT_TOKEN: mint with `grant <you> github-bot` in Devzat
DEVZAT_TOKEN=
# GITHUB_TOKEN: optional; raises API limits and enables private repos
GITHUB_TOKEN=
# GITHUB_DEFAULT_REPO: optional owner/repo so `gh pr 123` works without it
GITHUB_DEFAULT_REPO=
EOF
else
    echo "==> Keeping existing $ENV_FILE"
fi

# Upsert any values provided in the environment without clobbering the rest.
update_env() {
    local key="$1" val="${2:-}"
    [ -n "$val" ] || return 0
    if grep -q "^${key}=" "$ENV_FILE"; then
        sed -i "s|^${key}=.*|${key}=${val}|" "$ENV_FILE"
    else
        printf '%s=%s\n' "$key" "$val" >>"$ENV_FILE"
    fi
}
update_env DEVZAT_TOKEN "${DEVZAT_TOKEN:-}"
update_env GITHUB_TOKEN "${GITHUB_TOKEN:-}"
update_env GITHUB_DEFAULT_REPO "${GITHUB_DEFAULT_REPO:-}"
chmod 600 "$ENV_FILE"

if ! grep -q '^DEVZAT_TOKEN=.\+' "$ENV_FILE"; then
    echo
    echo "WARNING: DEVZAT_TOKEN is empty — the plugin will fail to start."
    echo "Grant one in Devzat (as admin):  grant <you> github-bot"
    echo "Then set it in $ENV_FILE (or pass DEVZAT_TOKEN=... to this script) and re-run."
fi

echo "==> Starting service"
systemctl daemon-reload
systemctl enable devzat-github
systemctl restart devzat-github

echo
systemctl --no-pager --full status devzat-github || true
