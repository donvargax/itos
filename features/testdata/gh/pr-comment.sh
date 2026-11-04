: > "$S/gh-commented"
url=
if [ -f "$S/gh-comment-prints" ]; then
  n=1
  if [ -f "$S/gh-comment-count" ]; then n=$(($(cat "$S/gh-comment-count") + 1)); fi
  echo "$n" > "$S/gh-comment-count"
  url=$(cat "$S/gh-comment-prints")$n
  printf '%s\n' "$url"
fi
if [ -f "$S/gh-comment-lists" ]; then
  at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  if [ -s "$S/gh-comments" ]; then printf ',' >> "$S/gh-comments"; fi
  printf '{"author":{"login":"someone"},"body":"%s","url":"%s","createdAt":"%s","updatedAt":"%s"}' \
    "${rec#*--body=}" "$url" "$at" "$at" >> "$S/gh-comments"
fi
