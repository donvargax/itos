case "$2" in
*/activity\?*)
  branch=${2#*ref=refs/heads/}
  branch=${branch%%&*}
  if [ -f "$S/gh-activity" ]; then
    after=$("$GIT" --git-dir="$REMOTE" rev-parse "refs/heads/$branch") || exit 1
    printf '[{"after":"%s","timestamp":"%s"}]\n' "$after" "$(cat "$S/gh-activity")"
  else
    printf '[]\n'
  fi ;;
esac
