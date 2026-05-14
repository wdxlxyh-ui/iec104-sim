#!/usr/bin/env bash
# ==========================================================================
# IEC104 Simulator - Automated Build Script
# ==========================================================================
# Usage:
#   ./build.sh                  Full build (all platforms + frontend)
#   ./build.sh --skip-web       Skip frontend rebuild (use existing web/dist/)
#   ./build.sh --fast           Skip type-check + skip web if unchanged
#   ./build.sh --help           Show help
#
# Output: dist/*.tar.gz (Linux) + dist/*.zip (Windows)
# ==========================================================================
set -euo pipefail

# ─── Config ───────────────────────────────────────────────────────────────
PROJECT="iec104-sim"
VERSION="2.1.3"
ROOT="$(cd "$(dirname "$0")" && pwd)"
DIST_DIR="$ROOT/dist"
GO_CMD="${GO:-go}"
PLATFORMS=(
    "linux:amd64:linux-amd64"
    "linux:arm64:linux-arm64"
    "windows:amd64:windows-amd64"
)

# ─── Flags ────────────────────────────────────────────────────────────────
SKIP_WEB=false
FAST=false

usage() {
    sed -n 's/^# //p; s/^#$//p; /^[^#]/q' "$0"
    exit 0
}

# ─── Parse args ───────────────────────────────────────────────────────────
for arg in "$@"; do
    case "$arg" in
        --skip-web) SKIP_WEB=true ;;
        --fast)     FAST=true ;;
        --help)     usage ;;
        *)          echo "Unknown: $arg"; usage ;;
    esac
done

# ─── Timing helper ────────────────────────────────────────────────────────
START_EPOCH=$(date +%s)
SECONDS=0
step_done() {
    local elapsed=$SECONDS
    echo "  ✔ $1  (${elapsed}s)"
}

# ─── 0. Auto-detect Go ────────────────────────────────────────────────────
# Try common Go installation paths if GO is not already set
if ! command -v "$GO_CMD" &>/dev/null; then
    for candidate in /usr/local/go/bin/go /home/hermes/go-toolchain/go/bin/go \
                     /snap/go/current/bin/go /opt/homebrew/bin/go; do
        if [ -x "$candidate" ]; then
            GO_CMD="$candidate"
            break
        fi
    done
fi
export PATH="$(dirname "$GO_CMD"):$PATH"

echo "═══════════════════════════════════════════════════════════════════"
echo "  $PROJECT v$VERSION Build"
echo "═══════════════════════════════════════════════════════════════════"
echo ""

# Check Go
if ! command -v "$GO_CMD" &>/dev/null; then
    echo "✖ Go not found. Set GO env var or install Go 1.21+"
    exit 1
fi
echo "  Go:   $($GO_CMD version 2>/dev/null || echo 'not found')"

# Check npm (only if building frontend)
if [ "$SKIP_WEB" = false ]; then
    if ! command -v npm &>/dev/null; then
        echo "✖ npm not found. Use --skip-web to build binary only."
        exit 1
    fi
    echo "  npm:  $(npm --version)"
fi
echo ""

# ─── 1. Frontend (Vue3) ──────────────────────────────────────────────────
BUILD_WEB=false
if [ "$SKIP_WEB" = true ]; then
    echo "[1/3] Frontend  ── skipped (--skip-web)"
elif [ "$FAST" = true ] && [ -f "$ROOT/web/dist/index.html" ]; then
    echo "[1/3] Frontend  ── skipped (--fast, web/dist/ exists)"
else
    BUILD_WEB=true
    echo "[1/3] Frontend  ── building (vue-tsc + vite) ..."
    echo "      ⚠ This is the slowest step (~30-50s) because:"
    echo "         • vue-tsc type-checks ~1650 TypeScript modules"
    echo "         • Vite bundles them into production assets"
    echo "      💡 Tip: use --skip-web if only Go code changed,"
    echo "         or --fast to skip type-check on rebuilds."
fi

# ─── 2. Go binaries (parallel) ────────────────────────────────────────────
LDFLAGS="-ldflags=-s -w -X main.version=$VERSION"
echo "[2/3] Go builds ── compiling for ${#PLATFORMS[@]} platforms ..."

mkdir -p "$DIST_DIR/bin"

# Build all platforms in parallel
GO_BUILD_PIDS=()
for entry in "${PLATFORMS[@]}"; do
    IFS=":" read -r goos goarch suffix <<< "$entry"
    bin_name="$PROJECT"
    [ "$goos" = "windows" ] && bin_name="$PROJECT.exe"

    (
        GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
            $GO_CMD build "$LDFLAGS" \
            -o "$DIST_DIR/bin/$PROJECT-$suffix${bin_name#$PROJECT}" \
            "$ROOT/cmd/iec104-sim/"
        echo "    ✔ $goos/$goarch  →  $(ls -lh "$DIST_DIR/bin/$PROJECT-$suffix${bin_name#$PROJECT}" | awk '{print $5}')"
    ) &
    GO_BUILD_PIDS+=($!)
done

# Build frontend in parallel with Go (if needed)
if [ "$BUILD_WEB" = true ]; then
    (
        cd "$ROOT/web"
        if [ "$FAST" = true ]; then
            # Skip vue-tsc type-check, just vite build
            npx vite build 2>/dev/null
        else
            npm install --silent 2>/dev/null
            npm run build 2>/dev/null
        fi
        echo "    ✔ Frontend built  →  web/dist/ ($(du -sh dist | cut -f1))"
    ) &
    WEB_PID=$!
else
    WEB_PID=""
fi

# Wait for Go builds
echo "    Waiting for Go builds ..."
failed=0
for pid in "${GO_BUILD_PIDS[@]}"; do
    wait "$pid" || failed=$((failed + 1))
done
if [ "$failed" -gt 0 ]; then
    echo "  ✖ $failed Go build(s) failed!"
    exit 1
fi
GO_END=$SECONDS
echo "    Go builds complete (${GO_END}s)"
echo ""

# ─── 3. Wait for frontend if building ────────────────────────────────────
if [ -n "$WEB_PID" ]; then
    echo "[3/3] Waiting for frontend to finish ..."
    wait "$WEB_PID" || { echo "  ✖ Frontend build failed"; exit 1; }
    echo ""
fi

# ─── 4. Package ───────────────────────────────────────────────────────────
echo "═══════════════════════════════════════════════════════════════════"
echo "  Packaging"
echo "═══════════════════════════════════════════════════════════════════"

# Check if web/dist exists (skip packaging if no frontend)
if [ ! -f "$ROOT/web/dist/index.html" ] && [ "$SKIP_WEB" = false ]; then
    echo "⚠ web/dist/ not found. Building frontend is required for web UI."
    echo "  Run: cd web && npm install && npm run build"
    echo "  Or use: ./build.sh --skip-web (binary only, no web UI)"
fi

PACKAGES=0
for entry in "${PLATFORMS[@]}"; do
    IFS=":" read -r goos goarch suffix <<< "$entry"

    STAGING="$DIST_DIR/staging/$PROJECT-v$VERSION-$suffix"
    mkdir -p "$STAGING/bin" "$STAGING/scripts" "$STAGING/config" \
             "$STAGING/logs" "$STAGING/web/dist" "$STAGING/samples"

    # Binary
    if [ "$goos" = "windows" ]; then
        cp "$DIST_DIR/bin/$PROJECT-$suffix.exe" "$STAGING/bin/$PROJECT.exe"
    else
        cp "$DIST_DIR/bin/$PROJECT-$suffix" "$STAGING/bin/$PROJECT"
        chmod +x "$STAGING/bin/$PROJECT"
    fi

    # Scripts
    if [ "$goos" = "windows" ]; then
        cp "$ROOT/scripts/start.bat"   "$STAGING/scripts/"
        cp "$ROOT/scripts/stop.bat"    "$STAGING/scripts/"
        cp "$ROOT/scripts/restart.bat" "$STAGING/scripts/"
    else
        cp "$ROOT/scripts/start.sh"    "$STAGING/scripts/"
        cp "$ROOT/scripts/stop.sh"     "$STAGING/scripts/"
        cp "$ROOT/scripts/restart.sh"  "$STAGING/scripts/"
        chmod +x "$STAGING/scripts/"*.sh
    fi

    # Config / samples
    echo '[]' > "$STAGING/config/instances.json"
    [ -f "$ROOT/samples/point.xlsx" ] && cp "$ROOT/samples/point.xlsx" "$STAGING/samples/"
    touch "$STAGING/logs/.gitkeep"

    # README
    [ -f "$ROOT/README.md" ] && cp "$ROOT/README.md" "$STAGING/"

    # Frontend assets (if available)
    if [ -d "$ROOT/web/dist" ]; then
        cp -r "$ROOT/web/dist/"* "$STAGING/web/dist/"
    fi

    # Compress
    cd "$DIST_DIR/staging"
    if [ "$goos" = "windows" ]; then
        if command -v zip &>/dev/null; then
            zip -rq "$DIST_DIR/$PROJECT-v$VERSION-$suffix.zip" \
                "$PROJECT-v$VERSION-$suffix/"
        else
            python3 -c "
import zipfile, os
src = '$PROJECT-v$VERSION-$suffix'
dst = '$DIST_DIR/$PROJECT-v$VERSION-$suffix.zip'
with zipfile.ZipFile(dst, 'w', zipfile.ZIP_DEFLATED) as zf:
    for root, dirs, files in os.walk(src):
        for f in files:
            p = os.path.join(root, f)
            zf.write(p, os.path.relpath(p, os.path.dirname(src)))
"
        fi
        echo "  ✔ iec104-sim-v$VERSION-$suffix.zip  ($(ls -lh "$DIST_DIR/$PROJECT-v$VERSION-$suffix.zip" | awk '{print $5}'))"
    else
        tar czf "$DIST_DIR/$PROJECT-v$VERSION-$suffix.tar.gz" \
            "$PROJECT-v$VERSION-$suffix/"
        echo "  ✔ iec104-sim-v$VERSION-$suffix.tar.gz  ($(ls -lh "$DIST_DIR/$PROJECT-v$VERSION-$suffix.tar.gz" | awk '{print $5}'))"
    fi
    PACKAGES=$((PACKAGES + 1))
done

# Cleanup staging
rm -rf "$DIST_DIR/staging"

# ─── Summary ──────────────────────────────────────────────────────────────
TOTAL=$SECONDS
echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "  Build complete  (${TOTAL}s)"
echo "═══════════════════════════════════════════════════════════════════"
ls -lh "$DIST_DIR/"*.tar.gz "$DIST_DIR/"*.zip 2>/dev/null | \
    awk '{printf "  %s  %s\n", $5, $9}'
echo ""
echo "  Total: $PACKAGES packages"
echo ""

# ─── Timing breakdown ─────────────────────────────────────────────────────
if [ "$BUILD_WEB" = true ]; then
    echo "  ⏱  Frontend (vue-tsc + vite):  built concurrently with Go"
fi
echo "  ⏱  Go cross-compile:              ${GO_END}s"
echo "  ⏱  Total wall clock:              ${TOTAL}s"
echo ""
echo "  💡 Want faster?  Use --skip-web if only Go code changed,"
echo "     or --fast to skip vue-tsc type-checking on frontend rebuilds."
