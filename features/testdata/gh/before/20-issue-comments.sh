case "$1 $2 $rec" in
"api repos/"*"/issues/"*"/comments "*" -f body="*)
  n=${2%/comments}
  n=${n##*/}
  address=
  if [ -f "$S/gh-pull-requests" ]; then
    while read -r b url rest; do
      if [ "${url##*/}" = "$n" ]; then address=$url; fi
    done < "$S/gh-pull-requests"
  fi
  printf '{"id":1,"html_url":"%s#issuecomment-1"}\n' "$address"
  exit 0 ;;
esac
