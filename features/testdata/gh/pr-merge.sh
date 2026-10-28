auto=
for a in "$@"; do
  if [ "$a" = --auto ]; then auto=1; fi
done
if [ -z "$auto" ] || [ -f "$S/gh-merges-at-once" ]; then
  date -u +%Y-%m-%dT%H:%M:%SZ > "$S/gh-merged-$3"
else
  : > "$S/gh-auto-merge-$3"
fi
