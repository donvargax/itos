if [ -n "$pageinfo" ]; then
  rep "$fields" '"comments":{"nodes":' "\"comments\":{$pageinfo\"nodes\":"
  fields=$REPLY
fi
