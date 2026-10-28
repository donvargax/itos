case "$rec" in
*" -f id="*)
  id=${rec##* -f id=}
  id=${id%% *}
  if [ -f "$S/gh-node-$id" ]; then
    printf '{"data":{"node":%s}}\n' "$(cat "$S/gh-node-$id")"
    exit 0
  fi
  msg="Could not resolve to a node with the global id of '$id'"
  printf '{"data":{"node":null},"errors":[{"type":"NOT_FOUND","path":["node"],"message":"%s"}]}\n' "$msg"
  printf 'gh: %s\n' "$msg" >&2
  exit 1 ;;
esac
