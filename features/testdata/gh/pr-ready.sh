: > "$S/gh-pull-requests.new"
while read -r branch url kind draft title; do
  if [ "$branch" = "$3" ]; then draft=false; fi
  printf '%s %s %s %s %s\n' "$branch" "$url" "$kind" "${draft:-false}" "$title" >> "$S/gh-pull-requests.new"
done < "$S/gh-pull-requests"
mv "$S/gh-pull-requests.new" "$S/gh-pull-requests"
