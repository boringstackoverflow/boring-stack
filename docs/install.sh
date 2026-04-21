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
#   curl -fsSL https://boringstackoverflow.github.io/boring-stack/install.sh | bash
#
# To install into a different home:
#   BORING_STACK_HOME=/opt/boring-stack curl -fsSL ... | bash

set -euo pipefail

REPO_URL="${BORING_STACK_REPO:-https://github.com/boringstackoverflow/boring-stack.git}"
INSTALL_DIR="${BORING_STACK_HOME:-$HOME/.boring-stack}"
SOURCE_FILE="$INSTALL_DIR/SKILL.md"

step() { printf '\033[36m→\033[0m %s\n' "$*"; }
ok()   { printf '\033[32m✓\033[0m %s\n' "$*"; }
warn() { printf '\033[33m!\033[0m %s\n' "$*" >&2; }

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
# 2. Wire into every user-level AI tool dir we find
#-----------------------------------------------------------------------------

INSTALLED=()

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
the 7 manifesto principles and the 4-question stack-picker intake.
EOF
    fi
    INSTALLED+=("Codex CLI    → $target_file")
fi

# Aider — user-level via ~/.aider.conf.yml read-history is project-level only,
# but ~/.aider.conf.yml CAN reference a global conventions file. Skip clean
# user-level for Aider; covered via project-level add.sh.

#-----------------------------------------------------------------------------
# 3. Report
#-----------------------------------------------------------------------------

echo
if [ ${#INSTALLED[@]} -eq 0 ]; then
    warn "no AI coding tool with user-level config detected on this machine"
    echo "  (looked for: ~/.claude, ~/.codex)"
    echo "  the skill is cloned at $INSTALL_DIR; you can still use it project-level"
    echo "  via the add.sh script below."
else
    ok "installed user-level for ${#INSTALLED[@]} tool(s):"
    for t in "${INSTALLED[@]}"; do echo "    $t"; done
fi

cat <<EOF

──────────────────────────────────────────────────────────────────────────
Project-level tools

Most AI coding tools (Cursor, Copilot, Cline, Aider, Gemini, Windsurf,
Continue, Zed) only support project-level rules. To add boring-stack
defaults to a specific project, cd into it and run:

    curl -fsSL https://boringstackoverflow.github.io/boring-stack/add.sh | bash

The script auto-detects which tools the project uses and drops the right
file in each. Falls back to AGENTS.md (the portable convention) if the
project has no AI-tool config yet.

──────────────────────────────────────────────────────────────────────────
Updating

Re-run this installer any time:

    curl -fsSL https://boringstackoverflow.github.io/boring-stack/install.sh | bash

Or pull manually:

    git -C $INSTALL_DIR pull

──────────────────────────────────────────────────────────────────────────
Try it

In Claude Code, type:    /boring-stack
In Codex CLI, the rules load automatically for new sessions.

EOF
