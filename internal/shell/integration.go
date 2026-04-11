package shell

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	EmitCDEnv      = "FT_EMIT_CD"
	EmitCDValue    = "1"
	CDMarkerPrefix = "__FT_CD__="
)

func EmitCDOrWarning(path string, stdout io.Writer, stderr io.Writer) {
	if strings.TrimSpace(path) == "" {
		return
	}

	if os.Getenv(EmitCDEnv) == EmitCDValue {
		if _, err := fmt.Fprintf(stdout, "%s%s\n", CDMarkerPrefix, path); err != nil {
			return
		}
		return
	}

	hint := PreferredShell()
	if _, err := fmt.Fprintln(stderr, "ft: shell integration not active, so automatic directory switching is unavailable"); err != nil {
		return
	}
	if _, err := fmt.Fprintf(stderr, "ft: run `eval \"$(ft init %s)\"` to enable auto-cd in this shell\n", hint); err != nil {
		return
	}
}

func InitScript(shellName string) (string, error) {
	switch shellName {
	case "zsh", "bash":
		return "# ft shell integration for " + shellName + "\n" + shellFunctionScript(), nil
	default:
		return "", fmt.Errorf("ft: unsupported shell %q (supported: bash, zsh)", shellName)
	}
}

func shellFunctionScript() string {
	return `ft() {
  local tmp_stdout exit_code cd_target line

  tmp_stdout="$(mktemp -t ft.stdout.XXXXXX)" || {
    printf 'ft: failed to create temporary file\n' >&2
    return 1
  }

  FT_EMIT_CD=1 command ft "$@" >"$tmp_stdout"
  exit_code=$?

  cd_target=""
  while IFS= read -r line; do
    case "$line" in
      __FT_CD__=*)
        cd_target="${line#__FT_CD__=}"
        ;;
      *)
        printf '%s\n' "$line"
        ;;
    esac
  done <"$tmp_stdout"

  rm -f "$tmp_stdout"

  if [ "$exit_code" -ne 0 ]; then
    return "$exit_code"
  fi

  if [ -n "$cd_target" ]; then
    cd "$cd_target" || return 1
  fi

  return 0
}
`
}

func PreferredShell() string {
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		return "zsh"
	}
	base := filepath.Base(shellPath)
	if base == "bash" || base == "zsh" {
		return base
	}
	return "zsh"
}
