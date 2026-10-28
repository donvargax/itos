nl='
'
rep() {
  s=$1 out=
  while :; do
    case $s in
    *"$2"*) out=$out${s%%"$2"*}$3; s=${s#*"$2"} ;;
    *) out=$out$s; break ;;
    esac
  done
  REPLY=$out
}
json() { rep "$1" '\' '\\'; rep "$REPLY" '"' '\"'; }
rec=
sep=
for a in "$@"; do rep "$a" "$nl" '\n'; rec=$rec$sep$REPLY; sep=' '; done
printf '%s\n' "$rec" >> "$S/gh-calls"
if [ -f "$S/gh-failure" ]; then cat "$S/gh-failure" >&2; exit 1; fi
head=
state=open
json=
prev=
for a in "$@"; do
  if [ "$prev" = --head ]; then head=$a; fi
  if [ "$prev" = --state ]; then state=$a; fi
  if [ "$prev" = --json ]; then json=$a; fi
  prev=$a
done
if [ -f "$H/$1-$2.sh" ]; then . "$H/$1-$2.sh"; fi
