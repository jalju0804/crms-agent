#!/bin/sh
set -eu

BIN_SOURCE="${BIN_SOURCE:-./vm-metric-agent}"
CONFIG_SOURCE="${CONFIG_SOURCE:-./config.yaml}"
SERVICE_SOURCE="${SERVICE_SOURCE:-./vm-metric-agent.service}"
LOGROTATE_SOURCE="${LOGROTATE_SOURCE:-./vm-metric-agent.logrotate}"

if [ "$(id -u)" -ne 0 ]; then
  echo "install.sh must run as root" >&2
  exit 1
fi

if ! id vmmetric >/dev/null 2>&1; then
  useradd --system --home /var/lib/vm-metric-agent --shell /usr/sbin/nologin vmmetric
fi

install -d -o vmmetric -g vmmetric -m 0750 /var/lib/vm-metric-agent
install -d -o root -g root -m 0755 /etc/vm-metric-agent
install -m 0755 "$BIN_SOURCE" /usr/local/bin/vm-metric-agent
install -m 0644 "$CONFIG_SOURCE" /etc/vm-metric-agent/config.yaml
install -m 0644 "$SERVICE_SOURCE" /etc/systemd/system/vm-metric-agent.service
if [ -f "$LOGROTATE_SOURCE" ]; then
  install -m 0644 "$LOGROTATE_SOURCE" /etc/logrotate.d/vm-metric-agent
fi

systemctl daemon-reload
systemctl enable vm-metric-agent.service
systemctl restart vm-metric-agent.service
systemctl --no-pager --full status vm-metric-agent.service
