out=
case "$json" in
""|*headRefOid*)
  oid=$("$GIT" --git-dir="$REMOTE" rev-parse "refs/heads/$3") || exit 1
  if [ -f "$S/gh-stale-left" ]; then
    left=$(cat "$S/gh-stale-left")
    if [ "$left" -gt 0 ]; then
      echo $((left - 1)) > "$S/gh-stale-left"
      oid=$(cat "$S/gh-previous-$3")
    fi
  fi
  out="\"headRefOid\":\"$oid\"" ;;
esac
case "$json" in
*state*)
  merged=null
  state=OPEN
  if [ -f "$S/gh-merged-$3" ]; then
    state=MERGED
    merged="\"$(cat "$S/gh-merged-$3")\""
  fi
  out="${out:+$out,}\"state\":\"$state\",\"mergedAt\":$merged" ;;
esac
case "$json" in
*autoMergeRequest*)
  request=null
  if [ -f "$S/gh-auto-merge-$3" ]; then request='{"mergeMethod":"REBASE"}'; fi
  out="${out:+$out,}\"autoMergeRequest\":$request" ;;
esac
printf '{%s}\n' "$out"
