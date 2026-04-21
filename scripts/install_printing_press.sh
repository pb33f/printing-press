#!/bin/sh
set -eu

REPO_NAME="${REPO_NAME:-pb33f/printing-press}"
BINARY_NAME="${BINARY_NAME:-pp}"
ARCHIVE_NAME="${ARCHIVE_NAME:-printing-press}"
ISSUE_URL="${ISSUE_URL:-https://github.com/pb33f/printing-press/issues/new}"

if [ -n "${GITHUB_TOKEN:-}" ]; then
  GH_TOKEN="${GITHUB_TOKEN}"
else
  GH_TOKEN="${GH_TOKEN:-}"
fi

TMP_DIR=""

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

say() {
  printf '%s\n' "$*"
}

setup_color() {
  if [ -t 1 ]; then
    MAGENTA=$(printf '\033[35m')
    RESET=$(printf '\033[m')
  else
    MAGENTA=""
    RESET=""
  fi
}

warn() {
  printf 'Warning: %s\n' "$*" >&2
}

die() {
  printf 'Error: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  if [ -n "${TMP_DIR}" ] && [ -d "${TMP_DIR}" ]; then
    rm -rf "${TMP_DIR}"
  fi
}

trap cleanup EXIT INT TERM

api_get() {
  if [ -n "${GH_TOKEN}" ]; then
    curl --fail --silent --show-error --location \
      -H "Authorization: Bearer ${GH_TOKEN}" \
      -H "X-GitHub-Api-Version: 2022-11-28" \
      "$1"
    return
  fi

  curl --fail --silent --show-error --location "$1"
}

latest_tag_from_api() {
  api_get "https://api.github.com/repos/${REPO_NAME}/releases/latest" |
    sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' |
    head -n 1
}

latest_tag_from_redirect() {
  curl --fail --silent --show-error --location --head --output /dev/null \
    --write-out '%{url_effective}' \
    "https://github.com/${REPO_NAME}/releases/latest" |
    sed 's#/*$##' |
    sed 's#.*/##'
}

resolve_latest_tag() {
  tag="$(latest_tag_from_api 2>/dev/null || true)"
  if [ -n "${tag}" ]; then
    printf '%s\n' "${tag}"
    return
  fi

  tag="$(latest_tag_from_redirect 2>/dev/null || true)"
  if [ -n "${tag}" ] && [ "${tag}" != "latest" ]; then
    printf '%s\n' "${tag}"
    return
  fi

  die "unable to determine the latest release for ${REPO_NAME}"
}

normalize_version() {
  printf '%s\n' "$1" | sed 's/^v//'
}

detect_os() {
  case "$(uname -s)" in
    Linux) printf 'linux\n' ;;
    Darwin) printf 'darwin\n' ;;
    *)
      die "$(uname -s) is not supported by this installer. Please open an issue: ${ISSUE_URL}"
      ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'x86_64\n' ;;
    arm64|aarch64) printf 'arm64\n' ;;
    *)
      die "$(uname -m) is not supported by this installer. Please open an issue: ${ISSUE_URL}"
      ;;
  esac
}

default_install_dir() {
  if [ -n "${INSTALL_DIR:-}" ]; then
    printf '%s\n' "${INSTALL_DIR}"
    return
  fi

  if [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ]; then
    printf '/usr/local/bin\n'
    return
  fi

  if [ -n "${HOME:-}" ]; then
    printf '%s\n' "${HOME}/.local/bin"
    return
  fi

  printf '/usr/local/bin\n'
}

ensure_install_dir() {
  target="$1"
  if [ ! -d "${target}" ]; then
    mkdir -p "${target}" 2>/dev/null || die "unable to create install directory ${target}. Set INSTALL_DIR to a writable directory."
  fi
  if [ ! -w "${target}" ]; then
    die "install directory ${target} is not writable. Set INSTALL_DIR to a writable directory."
  fi
}

asset_name() {
  printf '%s_%s_%s_%s.tar.gz\n' "${ARCHIVE_NAME}" "$1" "$2" "$3"
}

download_url() {
  printf 'https://github.com/%s/releases/download/%s/%s\n' "${REPO_NAME}" "$1" "$2"
}

checksum_url() {
  printf 'https://github.com/%s/releases/download/%s/checksums.txt\n' "${REPO_NAME}" "$1"
}

download_file() {
  url="$1"
  output="$2"
  curl --fail --silent --show-error --location --retry 3 --output "${output}" "${url}"
}

expected_checksum_for() {
  file="$1"
  checksums_file="$2"
  awk -v target="${file}" '$2 == target { print $1 }' "${checksums_file}"
}

actual_checksum_for() {
  file="$1"
  if command_exists sha256sum; then
    sha256sum "${file}" | awk '{print $1}'
    return
  fi
  if command_exists shasum; then
    shasum -a 256 "${file}" | awk '{print $1}'
    return
  fi
  if command_exists openssl; then
    openssl dgst -sha256 "${file}" | awk '{print $NF}'
    return
  fi
  die "no SHA-256 tool found. Install sha256sum, shasum, or openssl."
}

verify_checksum() {
  file="$1"
  checksums_file="$2"
  expected="$(expected_checksum_for "$(basename "${file}")" "${checksums_file}")"
  [ -n "${expected}" ] || die "unable to find checksum entry for $(basename "${file}")"

  actual="$(actual_checksum_for "${file}")"
  [ "${actual}" = "${expected}" ] || die "checksum mismatch for $(basename "${file}")"
}

warn_if_not_on_path() {
  target="$1"
  case ":${PATH:-}:" in
    *:"${target}":*)
      ;;
    *)
      warn "${target} is not on your PATH. You may need to add it before '${BINARY_NAME}' will work everywhere."
      ;;
  esac
}

main() {
  setup_color

  command_exists curl || die "curl is required"
  command_exists tar || die "tar is required"
  command_exists mktemp || die "mktemp is required"

  tag="${VERSION:-}"
  if [ -n "${tag}" ]; then
    case "${tag}" in
      v*) ;;
      *) tag="v${tag}" ;;
    esac
  else
    tag="$(resolve_latest_tag)"
  fi

  version="$(normalize_version "${tag}")"
  os="$(detect_os)"
  arch="$(detect_arch)"
  asset="$(asset_name "${version}" "${os}" "${arch}")"

  TARGET_DIR="$(default_install_dir)"
  ensure_install_dir "${TARGET_DIR}"
  warn_if_not_on_path "${TARGET_DIR}"

  TMP_DIR="$(mktemp -d)"
  archive_path="${TMP_DIR}/${asset}"
  checksums_path="${TMP_DIR}/checksums.txt"

  say "Installing ${BINARY_NAME} ${version}"
  say "Downloading $(download_url "${tag}" "${asset}")"

  download_file "$(download_url "${tag}" "${asset}")" "${archive_path}"
  download_file "$(checksum_url "${tag}")" "${checksums_path}"
  verify_checksum "${archive_path}" "${checksums_path}"

  tar -xzf "${archive_path}" -C "${TMP_DIR}"
  [ -f "${TMP_DIR}/${BINARY_NAME}" ] || die "archive did not contain ${BINARY_NAME}"

  chmod +x "${TMP_DIR}/${BINARY_NAME}"
  cp "${TMP_DIR}/${BINARY_NAME}" "${TARGET_DIR}/${BINARY_NAME}"

  say "Installed ${BINARY_NAME} to ${TARGET_DIR}/${BINARY_NAME}"
  "${TARGET_DIR}/${BINARY_NAME}" version || true

  printf '%s' "${MAGENTA}"
  cat <<EOF

@@@@@@@   @@@@@@@   @@@@@@   @@@@@@   @@@@@@@@
@@@@@@@@  @@@@@@@@  @@@@@@@  @@@@@@@  @@@@@@@@
@@!  @@@  @@!  @@@      @@@      @@@  @@!
!@!  @!@  !@   @!@      @!@      @!@  !@!
@!@@!@!   @!@!@!@   @!@!!@   @!@!!@   @!!!:!
!!@!!!    !!!@!!!!  !!@!@!   !!@!@!   !!!!!:
!!:       !!:  !!!      !!:      !!:  !!:
:!:       :!:  !:!      :!:      :!:  :!:
 ::        :: ::::  :: ::::  :: ::::   ::
 :        :: : ::    : : :    : : :    :

https://github.com/${REPO_NAME}
princess beef heavy industries presents: printing-press

Ta-da! printing-press is now installed! Run '${BINARY_NAME}' to start the printing press

EOF
  printf '%s' "${RESET}"
}

main "$@"
