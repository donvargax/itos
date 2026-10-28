if [ -f "$S/gh-ready-at" ]; then
  fields="\"timelineItems\":{\"nodes\":[{\"createdAt\":\"$(cat "$S/gh-ready-at")\"}]}"
fi
