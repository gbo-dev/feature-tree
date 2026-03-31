# Source this file from ~/.zshrc:
#   source /path/to/repo/wt.zsh

typeset -g _WT_WRAPPER_DIR="${${(%):-%x}:A:h}"

wt() {
  local script_path tmp_stdout exit_code cd_target line
  script_path="${_WT_WRAPPER_DIR}/wt"

  if [[ ! -x "$script_path" ]]; then
    printf 'wt: script not executable: %s\n' "$script_path" >&2
    return 1
  fi

  tmp_stdout="$(mktemp -t wt.stdout.XXXXXX)" || {
    printf 'wt: failed to create temporary file\n' >&2
    return 1
  }

  WT_EMIT_CD=1 "$script_path" "$@" >"$tmp_stdout"
  exit_code=$?

  cd_target=""
  while IFS= read -r line; do
    if [[ "$line" == __WT_CD__=* ]]; then
      cd_target="${line#__WT_CD__=}"
    else
      printf '%s\n' "$line"
    fi
  done < "$tmp_stdout"

  rm -f "$tmp_stdout"

  if [[ $exit_code -ne 0 ]]; then
    return $exit_code
  fi

  if [[ -n "$cd_target" ]]; then
    cd "$cd_target" || return 1
  fi

  return 0
}
