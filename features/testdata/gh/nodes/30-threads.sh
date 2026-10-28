threads=
if [ -f "$S/gh-threads" ]; then threads=$(cat "$S/gh-threads"); fi
fields="${fields:+$fields,}\"reviewThreads\":{\"nodes\":[$threads]}"
