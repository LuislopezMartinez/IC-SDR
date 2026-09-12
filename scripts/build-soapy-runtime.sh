#!/usr/bin/env bash
# Build a portable SoapySDR + RTL-SDR runtime into DATA/runtime/$GOOS-$GOARCH.
# Used on Linux and macOS so IC-SDR does not need distro/Homebrew SDR packages.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

GOOS_VAL="${GOOS:-}"
GOARCH_VAL="${GOARCH:-}"
if [[ -z "$GOOS_VAL" ]]; then
  case "$(uname -s)" in
    Darwin) GOOS_VAL=darwin ;;
    Linux) GOOS_VAL=linux ;;
    *) echo "unsupported host $(uname -s)" >&2; exit 1 ;;
  esac
fi
if [[ -z "$GOARCH_VAL" ]]; then
  case "$(uname -m)" in
    x86_64|amd64) GOARCH_VAL=amd64 ;;
    aarch64|arm64) GOARCH_VAL=arm64 ;;
    *) echo "unsupported arch $(uname -m)" >&2; exit 1 ;;
  esac
fi

if [[ "$GOOS_VAL" == "windows" ]]; then
  echo "Windows uses the existing ORIGEN SoapySDR runtime; skipping Unix bundle."
  exit 0
fi

prefix="$root/DATA/runtime/${GOOS_VAL}-${GOARCH_VAL}"
src="$root/build/soapy-src"
mkdir -p "$prefix" "$src"

LIBUSB_VER="1.0.27"
RTLSDR_VER="v2.0.2"
SOAPY_VER="soapy-sdr-0.8.1"
SOAPYRTL_VER="soapy-rtlsdr-0.3.0"

export PKG_CONFIG_PATH="$prefix/lib/pkgconfig${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}"
export CMAKE_PREFIX_PATH="$prefix${CMAKE_PREFIX_PATH:+:$CMAKE_PREFIX_PATH}"
export PATH="$prefix/bin:$PATH"

jobs="$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 2)"
cmake_arch=()
configure_host=()
libusb_opts=()
if [[ "$GOOS_VAL" == "darwin" ]]; then
  if [[ "$GOARCH_VAL" == "amd64" ]]; then
    export CFLAGS="${CFLAGS:-} -arch x86_64"
    export CXXFLAGS="${CXXFLAGS:-} -arch x86_64"
    export LDFLAGS="${LDFLAGS:-} -arch x86_64"
    cmake_arch=(-DCMAKE_OSX_ARCHITECTURES=x86_64)
    configure_host=(--host=x86_64-apple-darwin)
  else
    export CFLAGS="${CFLAGS:-} -arch arm64"
    export CXXFLAGS="${CXXFLAGS:-} -arch arm64"
    export LDFLAGS="${LDFLAGS:-} -arch arm64"
    cmake_arch=(-DCMAKE_OSX_ARCHITECTURES=arm64)
  fi
else
  libusb_opts=(--disable-udev)
fi

fetch() {
  local url="$1" dest="$2"
  if [[ -f "$dest" ]]; then
    return
  fi
  curl -L --fail --retry 3 -o "$dest" "$url"
}

extract() {
  local archive="$1" dest="$2"
  mkdir -p "$dest"
  tar -xf "$archive" -C "$dest" --strip-components=1
}

if [[ ! -f "$src/libusb/configure" ]]; then
  fetch "https://github.com/libusb/libusb/releases/download/v${LIBUSB_VER}/libusb-${LIBUSB_VER}.tar.bz2" "$src/libusb.tar.bz2"
  extract "$src/libusb.tar.bz2" "$src/libusb"
fi
if [[ ! -f "$src/rtl-sdr/CMakeLists.txt" ]]; then
  fetch "https://github.com/osmocom/rtl-sdr/archive/refs/tags/${RTLSDR_VER}.tar.gz" "$src/rtl-sdr.tar.gz"
  extract "$src/rtl-sdr.tar.gz" "$src/rtl-sdr"
fi
if [[ ! -f "$src/SoapySDR/CMakeLists.txt" ]]; then
  fetch "https://github.com/pothosware/SoapySDR/archive/refs/tags/${SOAPY_VER}.tar.gz" "$src/SoapySDR.tar.gz"
  extract "$src/SoapySDR.tar.gz" "$src/SoapySDR"
fi
if [[ ! -f "$src/SoapyRTLSDR/CMakeLists.txt" ]]; then
  fetch "https://github.com/pothosware/SoapyRTLSDR/archive/refs/tags/${SOAPYRTL_VER}.tar.gz" "$src/SoapyRTLSDR.tar.gz"
  extract "$src/SoapyRTLSDR.tar.gz" "$src/SoapyRTLSDR"
fi

echo "Building libusb ${LIBUSB_VER}"
pushd "$src/libusb" >/dev/null
if [[ ! -f Makefile ]]; then
  ./configure --prefix="$prefix" --disable-static "${libusb_opts[@]}" "${configure_host[@]}"
fi
make -j"$jobs"
make install
popd >/dev/null

echo "Building librtlsdr ${RTLSDR_VER}"
cmake -S "$src/rtl-sdr" -B "$src/rtl-sdr/build" \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_INSTALL_PREFIX="$prefix" \
  -DCMAKE_PREFIX_PATH="$prefix" \
  -DCMAKE_BUILD_WITH_INSTALL_RPATH=ON \
  -DINSTALL_UDEV_RULES=OFF \
  -DDETACH_KERNEL_DRIVER=ON \
  "${cmake_arch[@]}"
cmake --build "$src/rtl-sdr/build" --config Release -j"$jobs"
cmake --install "$src/rtl-sdr/build"

echo "Building SoapySDR ${SOAPY_VER}"
cmake -S "$src/SoapySDR" -B "$src/SoapySDR/build" \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_INSTALL_PREFIX="$prefix" \
  -DCMAKE_PREFIX_PATH="$prefix" \
  -DCMAKE_BUILD_WITH_INSTALL_RPATH=ON \
  -DENABLE_PYTHON=OFF \
  -DENABLE_PYTHON3=OFF \
  -DENABLE_DOCS=OFF \
  "${cmake_arch[@]}"
cmake --build "$src/SoapySDR/build" --config Release -j"$jobs"
cmake --install "$src/SoapySDR/build"

echo "Building SoapyRTLSDR ${SOAPYRTL_VER}"
cmake -S "$src/SoapyRTLSDR" -B "$src/SoapyRTLSDR/build" \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_INSTALL_PREFIX="$prefix" \
  -DCMAKE_PREFIX_PATH="$prefix" \
  -DCMAKE_BUILD_WITH_INSTALL_RPATH=ON \
  "${cmake_arch[@]}"
cmake --build "$src/SoapyRTLSDR/build" --config Release -j"$jobs"
cmake --install "$src/SoapyRTLSDR/build"

if [[ "$GOOS_VAL" == "linux" ]]; then
  if command -v patchelf >/dev/null 2>&1; then
    find "$prefix/lib" -type f \( -name '*.so' -o -name '*.so.*' \) -print0 |
      while IFS= read -r -d '' lib; do
        patchelf --set-rpath '$ORIGIN:$ORIGIN/../lib' "$lib" || true
      done
  fi
fi

if [[ "$GOOS_VAL" == "darwin" ]]; then
  export ICS_PREFIX="$prefix"
  python3 - <<'PY'
import os, pathlib, subprocess
prefix = pathlib.Path(os.environ["ICS_PREFIX"])
libs = list((prefix / "lib").glob("*.dylib"))
libs += list((prefix / "lib" / "SoapySDR").rglob("*.so"))
libs += list((prefix / "lib" / "SoapySDR").rglob("*.dylib"))
for lib in libs:
    subprocess.run(["install_name_tool", "-id", f"@rpath/{lib.name}", str(lib)], check=False)
    subprocess.run(["install_name_tool", "-add_rpath", "@loader_path", str(lib)], check=False)
    subprocess.run(["install_name_tool", "-add_rpath", "@loader_path/../lib", str(lib)], check=False)
print("updated macOS install names under", prefix)
PY
fi

echo "Installed SoapySDR runtime at $prefix"
find "$prefix" -type f \( -name '*.so*' -o -name '*.dylib' \) | sort
