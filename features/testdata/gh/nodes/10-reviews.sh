reviews=
if [ -f "$S/gh-reviews" ]; then reviews=$(cat "$S/gh-reviews"); fi
fields="${fields:+$fields,}\"reviews\":{\"nodes\":[$reviews]}"
