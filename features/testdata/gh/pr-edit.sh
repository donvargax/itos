title=
for a in "$@"; do
  case "$a" in --title=*) title=${a#--title=} ;; esac
done
: > "$S/gh-pull-requests.new"
while read -r branch url kind draft old; do
  if [ "$branch" = "$3" ] && [ "$kind" = OPEN ]; then old=$title; fi
  printf '%s %s %s %s %s\n' "$branch" "$url" "$kind" "${draft:-false}" "$old" >> "$S/gh-pull-requests.new"
done < "$S/gh-pull-requests"
mv "$S/gh-pull-requests.new" "$S/gh-pull-requests"
