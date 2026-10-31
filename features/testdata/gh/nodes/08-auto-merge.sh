n=${rec##*number=}
n=${n%% *}
request=null
if [ -f "$S/gh-pull-requests" ]; then
  while read -r b address rest; do
    if [ "${address##*/}" = "$n" ] && [ -f "$S/gh-auto-merge-$b" ]; then
      request='{"enabledAt":"2026-10-31T00:00:00Z"}'
    fi
  done < "$S/gh-pull-requests"
fi
fields="${fields:+$fields,}\"autoMergeRequest\":$request"
