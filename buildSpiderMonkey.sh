#!/usr/bin/env bash
set -euo pipefail

# Defaults - ARM64 RELEASE BUILD for native Apple Silicon
TAG="${TAG:-FIREFOX_33_1_1_RELEASE}"
ENABLE_JIT=1        # default: keep JITs off for stable decode/disasm
ENABLE_ASMJS=0
WITH_INTL=0
RUN_INSTALL=1

# Parse args
while [[ $# -gt 0 ]]; do
  case "$1" in
    --tag) TAG="$2"; shift 2 ;;
    --enable-jit) ENABLE_JIT=1; shift ;;
    --enable-asmjs) ENABLE_ASMJS=1; shift ;;
    --with-intl) WITH_INTL=1; shift ;;
    --no-install) RUN_INSTALL=0; shift ;;
    -h|--help)
      cat <<USAGE
Usage: $0 --tag FIREFOX_XX_Y_Z_RELEASE [--enable-jit] [--enable-asmjs] [--with-intl] [--no-install]

Builds SpiderMonkey ARM64 RELEASE version for a given mozilla-release tag natively.
Installs to ./dist/sm-<tag>-arm64 (relative to current directory).

Examples:
  $0 --tag FIREFOX_33_1_1_RELEASE
  $0 --tag FIREFOX_33_1_1_RELEASE --enable-jit
USAGE
      exit 0
      ;;
    *) echo "Unknown arg: $1"; exit 1 ;;
  esac
done

# Layout (relative to where you run the script)
ROOT="$(pwd)"
GECKO="${ROOT}/mozilla-release"            # shared repo
JSDIR="${GECKO}/js/src"
BUILDDIR="${JSDIR}/build-${TAG}-arm64"     # per-tag build dir for ARM64
DIST="${ROOT}/dist/sm-${TAG}-arm64"        # per-tag install prefix for ARM64

MARK='/* OOM override injected for mac build */'

echo "==> Root:      ${ROOT}"
echo "==> Repo:      ${GECKO}"
echo "==> Build dir: ${BUILDDIR}"
echo "==> Prefix:    ${DIST}"
echo "==> Tag:       ${TAG} (ARM64 RELEASE BUILD)"

# Must run natively on ARM64
if [[ "$(uname -m)" != "arm64" ]]; then
  echo "ERROR: not running on ARM64 (uname -m != arm64)"
  echo "This script builds for native ARM64. Current arch: $(uname -m)"
  exit 1
fi

# Tooling checks
command -v hg >/dev/null || { echo "Install Mercurial: brew install mercurial"; exit 1; }
AC213="$(command -v autoconf213 || true)"; [[ -z "${AC213}" ]] && AC213="$(command -v autoconf2.13 || true)"
[[ -z "${AC213}" ]] && { echo "Install autoconf 2.13: brew install autoconf@2.13"; exit 1; }
command -v pyenv >/dev/null || { echo "Install pyenv: brew install pyenv"; exit 1; }

# pyenv init + Python 2.7
export PYENV_ROOT="${HOME}/.pyenv"
export PATH="${PYENV_ROOT}/bin:${PATH}"
eval "$(pyenv init -)"
pyenv versions --bare | grep -qx 2.7.18 || pyenv install 2.7.18

# Clone or refresh repo once (shared)
if [[ ! -d "${GECKO}/.hg" ]]; then
  hg clone https://hg.mozilla.org/releases/mozilla-release "${GECKO}"
fi
cd "${GECKO}"
hg pull -u
hg update -r "${TAG}"

# Generate configure for this vintage
cd "${JSDIR}"
"${AC213}"

# Build dir + Python 2.7 local
mkdir -p "${BUILDDIR}"
cd "${BUILDDIR}"
pyenv local 2.7.18
python2.7 -V

# Patch OOM macro (idempotent) — affects headers used by this build tree
SRC_UH=../../public/Utility.h
[[ -f "${SRC_UH}" ]] || { echo "Missing ${SRC_UH} (after autoconf)."; exit 1; }

# Normalize any define line into a function-like no-op
sed -i.bak -E 's@^(#\s*define\s+JS_OOM_POSSIBLY_FAIL)(\s*\(\))?(\s+.*)?$@\1() ((void)0)@' "${SRC_UH}"

# Append guard once
grep -qF "${MARK}" "${SRC_UH}" || cat >> "${SRC_UH}" <<EOF

${MARK}
#ifdef JS_OOM_POSSIBLY_FAIL
#  undef JS_OOM_POSSIBLY_FAIL
#endif
#define JS_OOM_POSSIBLY_FAIL() ((void)0)
EOF

# Clean just this build dir/prefix
make distclean || true
rm -f config.cache
rm -rf dist "${DIST}"

# Base env (Native ARM64 clang) with explicit tool paths
export CC="clang"
export CXX="clang++"
export HOST_CC="clang"
export HOST_CXX="clang++"
export AR="$(xcrun -f ar)"
export RANLIB="$(xcrun -f ranlib)"
export LIBTOOL="$(xcrun -f libtool)"
export MACOSX_DEPLOYMENT_TARGET=11.0

# ARM64 RELEASE BUILD FLAGS - optimized, no debug
export CFLAGS="-O2 -DNDEBUG -Wno-implicit-int -Wno-implicit-function-declaration -arch arm64"
export CXXFLAGS="${CFLAGS} -std=gnu++11"
export HOST_CFLAGS="${CFLAGS}"
export HOST_CXXFLAGS="${CXXFLAGS}"
export LDFLAGS="-arch arm64"

# Feature toggles derived from flags
ASMJS_FLAG="--disable-asmjs";  [[ "${ENABLE_ASMJS}" -eq 1 ]] && ASMJS_FLAG="--enable-asmjs"
INTL_FLAG="--without-intl-api";[[ "${WITH_INTL}" -eq 1    ]] && INTL_FLAG="--with-intl-api"
# For these old revs, leaving JITs OFF is safest for decode/disasm tooling.
ION_FLAG="--disable-ion";       [[ "${ENABLE_JIT}" -eq 1   ]] && ION_FLAG="--enable-ion"
BASELINE_FLAG="--disable-baseline"; [[ "${ENABLE_JIT}" -eq 1 ]] && BASELINE_FLAG="--enable-baseline"

# Configure (prefix is per-tag) - ARM64 RELEASE VERSION
../configure \
  --prefix="${DIST}" \
  --target=aarch64-apple-darwin \
  --disable-debug \
  --enable-optimize \
  ${INTL_FLAG} \
  ${ASMJS_FLAG} \
  ${ION_FLAG} \
  ${BASELINE_FLAG} \
  --disable-tests \
  --disable-shared-js

# Build and (optionally) install
make -j"$(sysctl -n hw.ncpu)" V=1
if [[ "${RUN_INSTALL}" -eq 1 ]]; then
  make install
  echo "✔ Installed SpiderMonkey ARM64 RELEASE for ${TAG} to: ${DIST}"
fi

echo "Done."