#!/usr/bin/env bash
# Build libtetradec and install four timeslot copies beside the IC-SDR data tree.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

source_dir="${1:-third_party/libtetradec}"
build_dir="${2:-build/libtetradec}"
dest_dir="${3:-DATA/tools/tetra/runtime/bin}"

if [[ ! -f "$source_dir/CMakeLists.txt" ]]; then
  echo "libtetradec sources missing at $source_dir" >&2
  echo "Run: git submodule update --init --recursive" >&2
  exit 1
fi

mkdir -p "$build_dir" "$dest_dir"
cmake -S "$source_dir" -B "$build_dir" -DCMAKE_BUILD_TYPE=Release
cmake --build "$build_dir" --config Release

lib=""
for cand in \
  "$build_dir/libtetradec.so" \
  "$build_dir/libtetradec.dylib" \
  "$build_dir/libtetradec.dll" \
  "$build_dir/Release/libtetradec.dll" \
  "$build_dir/Debug/libtetradec.dll"
do
  if [[ -f "$cand" ]]; then
    lib="$cand"
    break
  fi
done
if [[ -z "$lib" ]]; then
  echo "built libtetradec was not found under $build_dir" >&2
  exit 1
fi

base="$(basename "$lib")"
ext="${base##*.}"
stem="${base%.*}"
cp -f "$lib" "$dest_dir/$base"
for slot in 2 3 4; do
  cp -f "$lib" "$dest_dir/${stem}-ts${slot}.${ext}"
done
ls -l "$dest_dir"
