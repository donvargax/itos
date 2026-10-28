if [ -f "$S/gh-comments-left" ]; then
  left=$(cat "$S/gh-comments-left")
  if [ "$left" -gt 0 ]; then
    echo $((left - 1)) > "$S/gh-comments-left"
    comments=$(cat "$S/gh-comments-early")
  fi
fi
