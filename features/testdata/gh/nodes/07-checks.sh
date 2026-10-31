rollup=null
if [ -f "$S/gh-checks" ]; then rollup="{\"state\":\"$(cat "$S/gh-checks")\"}"; fi
fields="${fields:+$fields,}\"commits\":{\"nodes\":[{\"commit\":{\"statusCheckRollup\":$rollup}}]}"
