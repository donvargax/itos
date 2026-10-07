#!/usr/bin/env bash
# The container's egress allowlist. Baked into the image as /usr/local/sbin/itos-firewall,
# owned by root, and run as root through the one sudoers rule the image grants the container
# user (no arguments allowed). The image's entrypoint runs it on every container start, since
# iptables rules do not survive a restart; running it again re-applies the same list.
#
# It closes egress first and opens it after, so a failure leaves the container closed:
#   1. OUTPUT drops by default (IPv4 and IPv6); loopback and established flows pass.
#   2. dnsmasq, as the itos-dns user, is the container's only resolver. It forwards only the
#      names in ALLOWED_NAMES (anything else is refused, so DNS is no way out) and adds each
#      address it answers with to the ipset, so a CDN address that rotates is in the set
#      before the client that asked for it connects. Only itos-dns may reach the upstream.
#   3. GitHub's published ranges (api.github.com/meta: web, api, git) go into the set too,
#      so git over SSH and HTTPS reaches GitHub whatever address its names answer with.
#   4. The set is accepted; everything else is rejected.
set -euo pipefail
PATH=/usr/sbin:/usr/bin:/sbin:/bin
export PATH

if [ "$(id -u)" -ne 0 ]; then
	echo "itos-firewall: run it as root (sudo /usr/local/sbin/itos-firewall)" >&2
	exit 1
fi
if [ $# -ne 0 ]; then
	echo "itos-firewall: takes no arguments; the allowlist is fixed in the image" >&2
	exit 1
fi

# Each name, and every name under it. Why each is here is in .devcontainer/README.md.
ALLOWED_NAMES=(
	github.com
	api.anthropic.com
	proxy.golang.org
	sum.golang.org
	vuln.go.dev
	registry.npmjs.org
	pypi.org
	files.pythonhosted.org
)
SET=allowed-egress
STATE=/var/lib/itos-firewall
RUN_DIR=/run/itos-firewall

# 1. Closed by default, from the first step.
iptables -P OUTPUT DROP
iptables -F OUTPUT
iptables -A OUTPUT -o lo -j ACCEPT
iptables -A OUTPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
if ip6tables -P OUTPUT DROP 2>/dev/null; then
	ip6tables -F OUTPUT
	ip6tables -A OUTPUT -o lo -j ACCEPT
	ip6tables -A OUTPUT -j REJECT
else
	echo "itos-firewall: no ip6tables in this container; IPv6 is not filtered" >&2
fi
ipset create "$SET" hash:net -exist
ipset flush "$SET"

# 2. The upstream resolvers: the ones Docker gave the container, saved the first time, since
#    the resolv.conf this script writes names dnsmasq instead.
mkdir -p "$STATE" "$RUN_DIR"
if ! grep -qx 'nameserver 127.0.0.1' /etc/resolv.conf; then
	cp /etc/resolv.conf "$STATE/resolv.conf"
fi
if [ ! -s "$STATE/resolv.conf" ]; then
	echo "itos-firewall: no upstream resolver saved; egress stays closed" >&2
	exit 1
fi
mapfile -t upstreams < <(awk '/^nameserver/ && $2 ~ /^[0-9.]+$/ {print $2}' "$STATE/resolv.conf")
if [ ${#upstreams[@]} -eq 0 ]; then
	echo "itos-firewall: no IPv4 resolver in $STATE/resolv.conf; egress stays closed" >&2
	exit 1
fi
dns_user=itos-dns
for ns in "${upstreams[@]}"; do
	iptables -A OUTPUT -p udp -d "$ns" --dport 53 -m owner --uid-owner "$dns_user" -j ACCEPT
	iptables -A OUTPUT -p tcp -d "$ns" --dport 53 -m owner --uid-owner "$dns_user" -j ACCEPT
done
iptables -A OUTPUT -m set --match-set "$SET" dst -j ACCEPT
iptables -A OUTPUT -p tcp -j REJECT --reject-with tcp-reset
iptables -A OUTPUT -j REJECT --reject-with icmp-admin-prohibited

# A dnsmasq from an earlier run in this container goes first; it holds port 53.
if pkill -x -u "$dns_user" dnsmasq; then
	for _ in $(seq 50); do pgrep -x -u "$dns_user" dnsmasq >/dev/null || break; sleep 0.1; done
fi
args=(--conf-file=/dev/null --no-resolv --no-poll --listen-address=127.0.0.1 --bind-interfaces
	--user="$dns_user" --group="$dns_user" --pid-file="$RUN_DIR/dnsmasq.pid" --cache-size=1000)
names=$(printf '/%s' "${ALLOWED_NAMES[@]}")
for ns in "${upstreams[@]}"; do
	args+=(--server="$names/$ns")
done
args+=(--ipset="$names/$SET")
dnsmasq "${args[@]}"
{
	echo 'nameserver 127.0.0.1'
	grep -E '^(search|options)' "$STATE/resolv.conf" || true
} > /etc/resolv.conf

# 3. GitHub's ranges, IPv4, as GitHub publishes them now (they change rarely; the names in
#    the list cover a change between two starts).
if ! meta=$(curl -fsS --max-time 20 https://api.github.com/meta); then
	echo "itos-firewall: api.github.com/meta unreachable; GitHub is reachable by name only" >&2
	exit 1
fi
jq -r '(.web + .api + .git)[] | select(test("^[0-9]+\\.[0-9]+\\.[0-9]+\\.[0-9]+/[0-9]+$"))' <<<"$meta" |
	sort -u | while read -r cidr; do ipset add "$SET" "$cidr" -exist; done

echo "itos-firewall: egress allowlisted: GitHub's meta ranges and ${ALLOWED_NAMES[*]}"
