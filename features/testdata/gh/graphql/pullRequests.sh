case "$rec" in
*"pullRequests("*)
  q=$rec
  answer=
  sep=
  while :; do
    case $q in *"pullRequests("*) ;; *) break ;; esac
    before=${q%%"pullRequests("*}
    q=${q#*"pullRequests("}
    before=${before% }
    before=${before%:}
    alias=${before##* }
    args=${q%%")"*}
    t=${args#*headRefName: \"}
    branch=${t%%\"*}
    states=
    case $args in *"states: ["*) t=${args#*"states: ["}; states=${t%%]*} ;; esac
    nodes=
    if [ -f "$S/gh-pull-requests" ]; then
      while read -r b url kind draft title; do
        if [ "$b" != "$branch" ]; then continue; fi
        case "$states/$kind" in /*|*"$kind"*/*) ;; *) continue ;; esac
        merged=null
        if [ "$kind" = MERGED ]; then merged='"2026-10-24T10:00:00Z"'; fi
        node="{\"number\":${url##*/},\"state\":\"$kind\",\"mergedAt\":$merged}"
        if [ -n "$nodes" ]; then nodes="$node,$nodes"; else nodes=$node; fi
      done < "$S/gh-pull-requests"
    fi
    answer="$answer$sep\"$alias\":{\"nodes\":[$nodes]}"
    sep=,
  done
  printf '{"data":{"repository":{%s}}}\n' "$answer"
  exit 0 ;;
esac
