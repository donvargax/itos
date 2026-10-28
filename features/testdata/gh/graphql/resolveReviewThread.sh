case "$rec" in
*resolveReviewThread*)
  printf '%s\n' '{"data":{"resolveReviewThread":{"thread":{"isResolved":true}}}}'
  exit 0 ;;
esac
