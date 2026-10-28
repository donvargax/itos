oid=$("$GIT" --git-dir="$REMOTE" rev-parse "refs/heads/$3") || exit 1
if [ -f "$S/gh-stale-left" ]; then
  left=$(cat "$S/gh-stale-left")
  if [ "$left" -gt 0 ]; then
    echo $((left - 1)) > "$S/gh-stale-left"
    oid=$(cat "$S/gh-previous-$3")
  fi
fi
printf '{"headRefOid":"%s"}\n' "$oid"
