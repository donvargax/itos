n=${rec##*number=}
n=${n%% *}
if [ -f "$S/gh-pull-requests" ]; then
  while read -r b address rest; do
    if [ "${address##*/}" = "$n" ]; then
      if oid=$("$GIT" --git-dir="$REMOTE" rev-parse "refs/heads/$b" 2>/dev/null); then
        fields="${fields:+$fields,}\"headRefOid\":\"$oid\""
      fi
    fi
  done < "$S/gh-pull-requests"
fi
