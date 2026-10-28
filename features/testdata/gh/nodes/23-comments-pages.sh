pageinfo=
if [ -f "$S/gh-comments-page2" ]; then
  case "$rec" in
  *"cursor=page2"*)
    comments=$(cat "$S/gh-comments-page2")
    pageinfo='"pageInfo":{"hasNextPage":false,"endCursor":"end"},' ;;
  *) pageinfo='"pageInfo":{"hasNextPage":true,"endCursor":"page2"},' ;;
  esac
fi
