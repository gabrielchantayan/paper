#!/bin/sh

set -eu

repository="gabrielchantayan/paper"
version="${PAPER_VERSION:-${1:-}}"
install_dir="${PAPER_INSTALL_DIR:-${HOME}/.local/bin}"

if [ -z "${version}" ]; then
  latest_url=$(curl --fail --location --silent --show-error \
    --output /dev/null --write-out '%{url_effective}' \
    "https://github.com/${repository}/releases/latest")
  version=${latest_url##*/}
fi

if ! printf '%s\n' "${version}" | grep -Eq '^v?[0-9]+\.[0-9]+\.[0-9]+$'; then
  printf 'Invalid version %s; expected v1.2.3 or 1.2.3\n' "${version}" >&2
  exit 1
fi
case "${version}" in
  v*) tag="${version}"; release_version=${version#v} ;;
  *) tag="v${version}"; release_version=${version} ;;
esac

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) printf 'Paper supports macOS and Linux through this installer.\n' >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) printf 'Unsupported architecture: %s\n' "$(uname -m)" >&2; exit 1 ;;
esac

archive="paper_${release_version}_${os}_${arch}.tar.gz"
release_url="https://github.com/${repository}/releases/download/${tag}"
temporary=$(mktemp -d)
trap 'rm -rf "${temporary}"' EXIT HUP INT TERM

curl --fail --location --silent --show-error \
  --output "${temporary}/${archive}" "${release_url}/${archive}"
curl --fail --location --silent --show-error \
  --output "${temporary}/checksums.txt" "${release_url}/checksums.txt"

expected=$(awk -v file="./${archive}" '$2 == file { print $1 }' "${temporary}/checksums.txt")
if [ -z "${expected}" ]; then
  printf 'No checksum found for %s\n' "${archive}" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "${temporary}/${archive}" | awk '{ print $1 }')
else
  actual=$(shasum -a 256 "${temporary}/${archive}" | awk '{ print $1 }')
fi
if [ "${actual}" != "${expected}" ]; then
  printf 'Checksum mismatch for %s\n' "${archive}" >&2
  exit 1
fi

mkdir -p "${temporary}/package" "${install_dir}"
tar -xzf "${temporary}/${archive}" -C "${temporary}/package"
install -m 0755 "${temporary}/package/paper" "${install_dir}/paper"

printf 'Installed paper %s at %s/paper\n' "${release_version}" "${install_dir}"
case ":${PATH}:" in
  *":${install_dir}:"*) ;;
  *) printf 'Add %s to PATH before running paper.\n' "${install_dir}" ;;
esac
