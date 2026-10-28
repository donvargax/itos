n=${rec##*number=}
n=${n%% *}
if [ -f "$S/gh-pull-requests" ]; then
  while read -r b address rest; do
    if [ "${address##*/}" = "$n" ]; then fields="\"number\":$n,\"url\":\"$address\""; fi
  done < "$S/gh-pull-requests"
fi
