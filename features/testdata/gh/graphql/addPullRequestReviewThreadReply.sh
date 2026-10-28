case "$rec" in
*addPullRequestReviewThreadReply*)
  printf '%s\n' '{"data":{"addPullRequestReviewThreadReply":{"comment":{"url":"https://github.example/o/r/pull/7#discussion_r1"}}}}'
  exit 0 ;;
esac
