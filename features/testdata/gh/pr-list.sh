found=
if [ -f "$S/gh-pull-requests" ]; then
  while read -r branch url kind draft title; do
    if [ "$branch" = "$head" ]; then
      case "$state/$kind" in all/*|open/OPEN|merged/MERGED|closed/CLOSED) ;; *) continue ;; esac
      merged=null
      if [ "$kind" = MERGED ]; then merged='"2026-10-24T10:00:00Z"'; fi
      oid=
      case "$json" in *headRefOid*) if [ -f "$S/gh-previous-$branch" ]; then oid=",\"headRefOid\":\"$(cat "$S/gh-previous-$branch")\""; fi ;; esac
      case "$json" in *mergeCommit*)
        oid="$oid,\"mergeCommit\":null"
        if [ "$kind" = MERGED ] && [ -f "$S/gh-merge-commit-$branch" ]; then oid="$oid,\"mergeCommit\":{\"oid\":\"$(cat "$S/gh-merge-commit-$branch")\"}"; fi ;;
      esac
      if [ -n "$found" ]; then found="$found,"; fi
      json "$title"
      found="$found{\"number\":${url##*/},\"url\":\"$url\",\"title\":\"$REPLY\",\"state\":\"$kind\",\"isDraft\":${draft:-false},\"mergedAt\":$merged$oid}"
    fi
  done < "$S/gh-pull-requests"
fi
printf '[%s]\n' "$found"
