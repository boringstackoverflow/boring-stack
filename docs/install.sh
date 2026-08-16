#!/usr/bin/env bash
#
# boring-stack user-level install.
#
# Clones the skill to ~/.boring-stack and wires it into every AI coding tool
# that has a USER-LEVEL config dir on this machine (Claude Code, Codex CLI).
# For tools that only support PROJECT-LEVEL rules (Cursor, Copilot, Cline,
# Aider, Gemini, Windsurf, Continue, Zed) it prints a one-liner you run
# inside each project where you want the boring stack defaults.
#
# Idempotent: re-runs cleanly to update.
#
#   curl -fsSL https://boringstack.org/install.sh | bash
#
# To install into a different home:
#   BORING_STACK_HOME=/opt/boring-stack curl -fsSL ... | bash

set -euo pipefail

REPO_URL="${BORING_STACK_REPO:-https://github.com/boringstackoverflow/boring-stack.git}"
INSTALL_DIR="${BORING_STACK_HOME:-$HOME/.boring-stack}"
SOURCE_FILE="$INSTALL_DIR/SKILL.md"
BIN_DIR="${BORING_STACK_BIN_DIR:-$HOME/.local/bin}"
CLI_BIN="$BIN_DIR/boringstack"

EVENTS_URL="${BORING_STACK_EVENTS_URL:-https://api.boringstack.org/v1/events}"

step() { printf '\033[36m→\033[0m %s\n' "$*"; }
ok()   { printf '\033[32m✓\033[0m %s\n' "$*"; }
warn() { printf '\033[33m!\033[0m %s\n' "$*" >&2; }

# Set up front because `set -u` is on and the CLI build below is conditional.
cli_installed=false
revision=dev

# install_id identifies this installation, not this run, so re-running the
# installer to update does not look like a brand new install. Stored next to
# the checkout; delete the file and you become a new install.
install_id() {
    local f="$INSTALL_DIR/.install-id"
    if [ -f "$f" ]; then cat "$f" 2>/dev/null && return; fi
    local id
    id=$( (uuidgen 2>/dev/null || od -An -N16 -tx1 /dev/urandom 2>/dev/null || date +%s%N) \
          | tr -d ' \n-' | tr 'A-Z' 'a-z' | cut -c1-32 )
    mkdir -p "$INSTALL_DIR" 2>/dev/null || true
    printf '%s' "$id" > "$f" 2>/dev/null || true
    printf '%s' "$id"
}

# report sends one anonymous event: no username, no paths, no project names.
#
# Every guard here is deliberate. `--max-time 2` bounds a hung endpoint, `|| true`
# keeps `set -e` from turning a network blip into a failed install, and the
# redirects keep curl silent. An install must never fail, stall, or look broken
# because our analytics box is down. It runs last, after all user-facing output,
# so even the 2s worst case is invisible.
report() {
    local event="$1"; shift
    step "reporting anonymous install event to ${EVENTS_URL%/v1/events}"
    curl -fsS --max-time 2 -X POST "$EVENTS_URL" \
        -d "event=$event" \
        -d "install_id=$(install_id)" \
        -d "os=$(uname -s 2>/dev/null || echo unknown)" \
        -d "arch=$(uname -m 2>/dev/null || echo unknown)" \
        "$@" >/dev/null 2>&1 || true
}

#-----------------------------------------------------------------------------
# 1. Clone or update the canonical install
#-----------------------------------------------------------------------------

if [ -d "$INSTALL_DIR/.git" ]; then
    step "updating existing install at $INSTALL_DIR"
    git -C "$INSTALL_DIR" pull --ff-only --quiet
else
    step "cloning to $INSTALL_DIR"
    git clone --depth 1 --quiet "$REPO_URL" "$INSTALL_DIR"
fi

if [ ! -f "$SOURCE_FILE" ]; then
    warn "SKILL.md not found at $SOURCE_FILE — install may be broken"
    exit 1
fi

#-----------------------------------------------------------------------------
# 2. Build the local CLI (the generated apps require Go too)
#-----------------------------------------------------------------------------

INSTALLED=()

if command -v go >/dev/null 2>&1; then
    step "building boringstack CLI"
    mkdir -p "$BIN_DIR"
    revision=$(git -C "$INSTALL_DIR" rev-parse --short HEAD 2>/dev/null || echo dev)
    export BORING_STACK_REVISION="$revision"
    if (
        cd "$INSTALL_DIR"
        go build -trimpath -ldflags="-s -w -X main.version=$revision" \
            -o "$CLI_BIN" ./cmd/boringstack
    ); then
        cli_installed=true
        INSTALLED+=("CLI          → $CLI_BIN")
        case ":$PATH:" in
            *":$BIN_DIR:"*) ;;
            *)
                warn "$BIN_DIR is not on PATH"
                echo "  add this to your shell profile: export PATH=\"$BIN_DIR:\$PATH\""
                ;;
        esac
    else
        warn "CLI build failed; the AI skill is still installed"
        echo "  check 'go version' (Go 1.22+ required), then re-run this installer"
    fi
else
    warn "Go is not installed; skipping the boringstack CLI"
    echo "  the AI skill will still work. Install Go 1.22+ and re-run this installer"
fi

#-----------------------------------------------------------------------------
# 3. Wire into every user-level AI tool dir we find
#-----------------------------------------------------------------------------

# Claude Code (Anthropic CLI). Skills live at ~/.claude/skills/<name>/SKILL.md.
if [ -d "$HOME/.claude" ] || command -v claude >/dev/null 2>&1; then
    target_dir="$HOME/.claude/skills/boring-stack"
    mkdir -p "$target_dir"
    ln -sf "$SOURCE_FILE" "$target_dir/SKILL.md"
    INSTALLED+=("Claude Code  → $target_dir/SKILL.md")
fi

# OpenAI Codex CLI. User-level instructions at ~/.codex/instructions.md.
# Append a reference rather than replace, since users may have other rules.
if [ -d "$HOME/.codex" ] || command -v codex >/dev/null 2>&1; then
    mkdir -p "$HOME/.codex"
    target_file="$HOME/.codex/instructions.md"
    if [ ! -f "$target_file" ] || ! grep -q "boring-stack" "$target_file" 2>/dev/null; then
        cat >> "$target_file" <<EOF

# Boring Stack
For long-lived web-app projects (side projects, internal tools, indie
products), prefer the boring stack: Go binary + SQLite + Litestream + Caddy
+ systemd + a \$5 VPS. See $SOURCE_FILE for the full ruleset, including
the 7 manifesto principles and the 5-question stack-picker intake.
EOF
    fi
    INSTALLED+=("Codex CLI    → $target_file")
fi

# Aider — user-level via ~/.aider.conf.yml read-history is project-level only,
# but ~/.aider.conf.yml CAN reference a global conventions file. Skip clean
# user-level for Aider; covered via project-level add.sh.

#-----------------------------------------------------------------------------
# 4. Report
#-----------------------------------------------------------------------------

echo
if [ ${#INSTALLED[@]} -eq 0 ]; then
    warn "no AI coding tool with user-level config detected on this machine"
    echo "  (looked for: ~/.claude, ~/.codex)"
    echo "  the skill is cloned at $INSTALL_DIR; you can still use it project-level"
    echo "  via the add.sh script below."
else
    ok "installed ${#INSTALLED[@]} component(s):"
    for t in "${INSTALLED[@]}"; do echo "    $t"; done
fi

cat <<EOF

──────────────────────────────────────────────────────────────────────────
Project-level tools

Most AI coding tools (Cursor, Copilot, Cline, Aider, Gemini, Windsurf,
Continue, Zed) only support project-level rules. To add boring-stack
defaults to a specific project, cd into it and run:

    curl -fsSL https://boringstack.org/add.sh | bash

The script auto-detects which tools the project uses and drops the right
file in each. Falls back to AGENTS.md (the portable convention) if the
project has no AI-tool config yet.

──────────────────────────────────────────────────────────────────────────
Updating

Re-run this installer any time:

    curl -fsSL https://boringstack.org/install.sh | bash

Or pull manually:

    git -C $INSTALL_DIR pull

──────────────────────────────────────────────────────────────────────────
Try it

From a terminal:

    boringstack new myapp
    cd myapp
    boringstack dev

Or ask your coding agent:

    Build me an expense-tracking SaaS using Boring Stack.

In Claude Code, /boring-stack still loads the intake directly. In Codex CLI,
the rules load automatically for new sessions.

EOF

#-----------------------------------------------------------------------------
# 5. Anonymous install report (last, after everything the user cares about)
#-----------------------------------------------------------------------------
#
# "install_success" means the skill installed. It does NOT mean the CLI did:
# the CLI build is skipped entirely when Go is missing, so cli_installed is
# reported separately and the dashboard's CLI install rate counts only the
# true ones. Conflating the two would have quietly overstated adoption.

report install_success -d "cli_installed=$cli_installed" -d "rev=$revision"
