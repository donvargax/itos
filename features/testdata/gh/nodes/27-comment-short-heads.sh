if [ -f "$S/gh-comment-heads" ]; then
  while read -r branch; do
    sha=$("$GIT" --git-dir="$REMOTE" rev-parse "refs/heads/$branch") || continue
    rep "$comments" "@SHORT:$branch@" "$(printf '%.7s' "$sha")"
    comments=$REPLY
  done < "$S/gh-comment-heads"
fi
