draft=false
title=
for a in "$@"; do
  if [ "$a" = --draft ]; then draft=true; fi
  case "$a" in --title=*) title=${a#--title=} ;; esac
done
printf '%s %s OPEN %s %s\n' "$head" "$ADDRESS" "$draft" "$title" >> "$S/gh-pull-requests"
printf '%s\n' "$ADDRESS"
