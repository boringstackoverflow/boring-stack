#!/usr/bin/env bash
#
# boring-stack project-level install.
#
# Run this from inside a project directory. Detects which AI coding tools
# the project uses (by config file presence + repo conventions) and drops
# the boring-stack ruleset into the right place for each. Idempotent.
#
#   cd /path/to/your/project
#   curl -fsSL https://boringstackoverflow.github.io/boring-stack/add.sh | bash
#
# To force a specific tool (skip auto-detect):
#   curl -fsSL https://...add.sh | bash -s -- --tool cursor
#
# Recognized tools: claude, cursor, copilot, codex, aider, cline, continue,
#   gemini, windsurf, zed, agents (the portable AGENTS.md fallback).

set -euo pipefail

INSTALL_DIR="${BORING_STACK_HOME:-$HOME/.boring-stack}"
SOURCE="$INSTALL_DIR/SKILL.md"
RAW_URL="${BORING_STACK_RAW:-https://raw.githubusercontent.com/boringstackoverflow/boring-stack/main/SKILL.md}"
FORCE_TOOL=""

while [ $# -gt 0 ]; do
    case "$1" in
        --tool) FORCE_TOOL="${2:-}"; shift 2 ;;
        --help|-h)
            sed -n '2,17p' "$0" | sed 's/^# \{0,1\}//'
            exit 0 ;;
        *) echo "unknown flag: $1" >&2; exit 2 ;;
    esac
done

step() { printf '\033[36m→\033[0m %s\n' "$*"; }
ok()   { printf '\033[32m✓\033[0m %s\n' "$*"; }
skip() { printf '\033[2m·\033[0m %s\n' "$*"; }

#-----------------------------------------------------------------------------
# Source the SKILL.md content. Prefer the local install; fall back to fetch.
#-----------------------------------------------------------------------------

if [ ! -f "$SOURCE" ]; then
    step "boring-stack not installed at $INSTALL_DIR; fetching SKILL.md inline"
    SOURCE=$(mktemp)
    trap 'rm -f "$SOURCE"' EXIT
    curl -fsSL "$RAW_URL" > "$SOURCE"
fi

#-----------------------------------------------------------------------------
# Per-tool install actions
#-----------------------------------------------------------------------------

INSTALLED=()
SKIPPED=()

want() {
    # Returns 0 if tool $1 should be installed (forced OR auto-detected).
    local tool="$1" detected="$2"
    if [ -n "$FORCE_TOOL" ]; then
        [ "$FORCE_TOOL" = "$tool" ] || [ "$FORCE_TOOL" = "all" ]
    else
        [ "$detected" = "yes" ]
    fi
}

# Append a marked block to a file, idempotent on the marker.
append_block() {
    local file="$1" marker="$2"
    if [ -f "$file" ] && grep -q "$marker" "$file" 2>/dev/null; then
        return 1  # already present
    fi
    mkdir -p "$(dirname "$file")"
    {
        printf '\n\n%s\n\n' "$marker"
        cat "$SOURCE"
    } >> "$file"
    return 0
}

# Copy SKILL.md verbatim to a destination (overwrites — these are dedicated
# rule files, not user-edited).
copy_to() {
    local dest="$1"
    mkdir -p "$(dirname "$dest")"
    cp "$SOURCE" "$dest"
}

#-----------------------------------------------------------------------------
# Detect + install for each tool
#-----------------------------------------------------------------------------

# Claude Code project-level
detect=no; { [ -d ".claude" ] || [ -f "CLAUDE.md" ]; } && detect=yes
if want claude "$detect"; then
    copy_to ".claude/skills/boring-stack/SKILL.md"
    INSTALLED+=("Claude Code  → .claude/skills/boring-stack/SKILL.md")
fi

# Cursor
detect=no; { [ -d ".cursor" ] || [ -f ".cursorrules" ]; } && detect=yes
if want cursor "$detect"; then
    copy_to ".cursor/rules/boring-stack.mdc"
    INSTALLED+=("Cursor       → .cursor/rules/boring-stack.mdc")
fi

# GitHub Copilot
detect=no; { [ -f ".github/copilot-instructions.md" ] || [ -d ".github" ]; } && detect=yes
if want copilot "$detect"; then
    if append_block ".github/copilot-instructions.md" "<!-- boring-stack -->"; then
        INSTALLED+=("Copilot      → .github/copilot-instructions.md (appended)")
    else
        SKIPPED+=("Copilot      .github/copilot-instructions.md already includes boring-stack")
    fi
fi

# Cline (VS Code extension)
detect=no; [ -f ".clinerules" ] && detect=yes
if want cline "$detect"; then
    copy_to ".clinerules"
    INSTALLED+=("Cline        → .clinerules")
fi

# Continue.dev
detect=no; { [ -f ".continuerules" ] || [ -d ".continue" ]; } && detect=yes
if want continue "$detect"; then
    copy_to ".continuerules"
    INSTALLED+=("Continue.dev → .continuerules")
fi

# Windsurf
detect=no; [ -f ".windsurfrules" ] && detect=yes
if want windsurf "$detect"; then
    copy_to ".windsurfrules"
    INSTALLED+=("Windsurf     → .windsurfrules")
fi

# Aider — uses CONVENTIONS.md (file path is configurable, but this is the
# documented default in the Aider docs).
detect=no; { [ -f "CONVENTIONS.md" ] || [ -f ".aider.conf.yml" ]; } && detect=yes
if want aider "$detect"; then
    if append_block "CONVENTIONS.md" "<!-- boring-stack -->"; then
        INSTALLED+=("Aider        → CONVENTIONS.md (appended)")
    else
        SKIPPED+=("Aider        CONVENTIONS.md already includes boring-stack")
    fi
fi

# Gemini Code Assist
detect=no; [ -f "GEMINI.md" ] && detect=yes
if want gemini "$detect"; then
    if append_block "GEMINI.md" "<!-- boring-stack -->"; then
        INSTALLED+=("Gemini       → GEMINI.md (appended)")
    else
        SKIPPED+=("Gemini       GEMINI.md already includes boring-stack")
    fi
fi

# Zed
detect=no; [ -f ".rules" ] && detect=yes
if want zed "$detect"; then
    copy_to ".rules"
    INSTALLED+=("Zed          → .rules")
fi

# Codex CLI / generic — AGENTS.md is the portable convention several tools
# read from. Always install if forced; otherwise install only when nothing
# else was detected (so we have a fallback, not a duplicate).
detect=no
[ -f "AGENTS.md" ] && detect=yes
[ ${#INSTALLED[@]} -eq 0 ] && [ -z "$FORCE_TOOL" ] && detect=yes  # fallback
if want agents "$detect"; then
    if append_block "AGENTS.md" "<!-- boring-stack -->"; then
        INSTALLED+=("AGENTS.md    → AGENTS.md (appended; portable across tools)")
    else
        SKIPPED+=("AGENTS.md    already includes boring-stack")
    fi
fi

#-----------------------------------------------------------------------------
# Report
#-----------------------------------------------------------------------------

echo
if [ ${#INSTALLED[@]} -gt 0 ]; then
    ok "added boring-stack to ${#INSTALLED[@]} tool config(s) in $(basename "$PWD")/:"
    for t in "${INSTALLED[@]}"; do echo "    $t"; done
fi
if [ ${#SKIPPED[@]} -gt 0 ]; then
    echo
    for s in "${SKIPPED[@]}"; do skip "$s"; done
fi
if [ ${#INSTALLED[@]} -eq 0 ] && [ ${#SKIPPED[@]} -eq 0 ]; then
    echo "no AI tool configs detected and no --tool flag passed. Re-run with"
    echo "  --tool <name>  (claude|cursor|copilot|codex|aider|cline|continue|gemini|windsurf|zed|agents|all)"
fi

echo
echo "Update later by re-running this script. Tracked content lives at:"
echo "  $INSTALL_DIR/SKILL.md  (or fetched fresh from main if not installed)"
