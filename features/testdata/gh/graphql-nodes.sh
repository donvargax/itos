fields=
for f in "$H"/nodes/*.sh; do . "$f"; done
printf '{"data":{"repository":{"pullRequest":{%s}}}}\n' "$fields"
