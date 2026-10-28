if [ ! -f "$S/gh-update-keeps" ]; then
  rm -rf "$S/gh-rebase"
  "$GIT" clone -q --branch "$3" "$REMOTE" "$S/gh-rebase" &&
    "$GIT" -C "$S/gh-rebase" rebase -q origin/main &&
    "$GIT" -C "$S/gh-rebase" push -q --no-verify --force origin "HEAD:refs/heads/$3" || exit 1
fi
if [ -f "$S/gh-stale-calls" ]; then cp "$S/gh-stale-calls" "$S/gh-stale-left"; fi
