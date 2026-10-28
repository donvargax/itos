if [ -f "$S/gh-comments-before" ]; then
  extra=$(cat "$S/gh-comments-before")
  if [ -f "$S/gh-commented" ]; then extra=$(cat "$S/gh-comments-after"); fi
  if [ -n "$comments" ]; then comments="$comments,$extra"; else comments=$extra; fi
fi
