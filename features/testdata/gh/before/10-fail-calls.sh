if [ -f "$S/gh-fail-calls" ]; then
  tab=$(printf '\t')
  while IFS=$tab read -r cmd text words; do
    case "$rec" in
    "$cmd "*"$text"*) printf '%s\n' "$words" >&2; exit 1 ;;
    esac
  done < "$S/gh-fail-calls"
fi
