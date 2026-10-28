#!/bin/sh
set -eu

: "${NAS_HOST:?set NAS_HOST to the NAS address, e.g. 192.168.1.10}"
: "${MEDIA_MCP_TOKEN:?export MEDIA_MCP_TOKEN with the MCP_TOKEN from the NAS .env}"

here=$(cd "$(dirname "$0")" && pwd)
base="http://$NAS_HOST:9090"
media_url="$base/media/mcp"
jellyfin_url="$base/jellyfin/mcp"

install_skill() {
  target="$1/nas-media"
  mkdir -p "$target"
  cp "$here/skill/nas-media/SKILL.md" "$target/SKILL.md"
  echo "skill -> $target"
}

if command -v claude >/dev/null 2>&1; then
  for name in nas-media nas-jellyfin; do
    claude mcp remove --scope user "$name" >/dev/null 2>&1 || true
  done
  claude mcp add --scope user --transport http nas-media "$media_url" \
    --header "Authorization: Bearer $MEDIA_MCP_TOKEN"
  claude mcp add --scope user --transport http nas-jellyfin "$jellyfin_url" \
    --header "Authorization: Bearer $MEDIA_MCP_TOKEN"
  install_skill "$HOME/.claude/skills"
fi

if command -v codex >/dev/null 2>&1; then
  for name in nas-media nas-jellyfin; do
    codex mcp remove "$name" >/dev/null 2>&1 || true
  done
  codex mcp add nas-media --url "$media_url" --bearer-token-env-var MEDIA_MCP_TOKEN
  codex mcp add nas-jellyfin --url "$jellyfin_url" --bearer-token-env-var MEDIA_MCP_TOKEN
  install_skill "$HOME/.codex/skills"
  echo "codex reads the token from MEDIA_MCP_TOKEN: export it in your shell profile"
fi

if command -v agy >/dev/null 2>&1; then
  agy mcp add --header "Authorization: Bearer $MEDIA_MCP_TOKEN" nas-media "$media_url"
  agy mcp add --header "Authorization: Bearer $MEDIA_MCP_TOKEN" nas-jellyfin "$jellyfin_url"
  install_skill "$HOME/.gemini/antigravity-cli/skills"
fi
