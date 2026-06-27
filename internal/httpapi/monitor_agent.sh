#!/bin/bash
# oci-start-go monitor agent v2.0
# Reports system metrics (CPU, memory, disk, network) to the server.
# Usage:
#   ./monitor_agent.sh              # run in foreground
#   ./monitor_agent.sh install      # install as systemd service
#   ./monitor_agent.sh uninstall    # remove systemd service

# ================= Dynamic configuration (replaced by server) =================
SERVER_URL="{{SERVER_URL}}"
TOKEN="{{TOKEN}}"
INTERVAL={{INTERVAL}}
DEBUG=false
# =============================================================================

# Detect primary network interface
MAIN_INTERFACE=$(ip route | grep default | awk '{print $5}' | head -n1)
if [ -z "$MAIN_INTERFACE" ]; then
    MAIN_INTERFACE=$(cat /proc/net/dev | grep -v lo | head -n 2 | tail -n 1 | awk -F: '{print $1}' | sed 's/ //g')
fi

# Rate calculation state
PREV_CPU_TOTAL=0
PREV_CPU_IDLE=0
PREV_RX_BYTES=0
PREV_TX_BYTES=0

send_report() {
    # 1. System info
    HOSTNAME=$(hostname)
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS_NAME=$PRETTY_NAME
    else
        OS_NAME=$(uname -s)
    fi
    KERNEL=$(uname -r)
    UPTIME=$(awk '{print int($1)}' /proc/uptime)

    # 2. CPU
    CPU_INFO=$(grep '^cpu ' /proc/stat)
    CPU_USER=$(echo $CPU_INFO | awk '{print $2}')
    CPU_NICE=$(echo $CPU_INFO | awk '{print $3}')
    CPU_SYS=$(echo $CPU_INFO | awk '{print $4}')
    CPU_IDLE=$(echo $CPU_INFO | awk '{print $5}')
    CPU_IOWAIT=$(echo $CPU_INFO | awk '{print $6}')
    CPU_IRQ=$(echo $CPU_INFO | awk '{print $7}')
    CPU_SOFTIRQ=$(echo $CPU_INFO | awk '{print $8}')

    CPU_TOTAL=$((CPU_USER + CPU_NICE + CPU_SYS + CPU_IDLE + CPU_IOWAIT + CPU_IRQ + CPU_SOFTIRQ))

    DIFF_TOTAL=$((CPU_TOTAL - PREV_CPU_TOTAL))
    DIFF_IDLE=$((CPU_IDLE - PREV_CPU_IDLE))

    if [ $DIFF_TOTAL -eq 0 ]; then
        CPU_USAGE=0
    else
        CPU_USAGE=$(awk -v total=$DIFF_TOTAL -v idle=$DIFF_IDLE 'BEGIN {printf "%.1f", (1 - idle/total)*100}')
    fi

    PREV_CPU_TOTAL=$CPU_TOTAL
    PREV_CPU_IDLE=$CPU_IDLE

    CPU_CORES=$(grep -c ^processor /proc/cpuinfo)
    CPU_MODEL=$(grep "model name" /proc/cpuinfo | head -n 1 | awk -F': ' '{print $2}')
    LOAD_AVG=$(awk '{print $1", "$2", "$3}' /proc/loadavg)

    # 3. Memory (MB)
    MEM_TOTAL=$(grep MemTotal /proc/meminfo | awk '{printf "%d", $2/1024}')
    MEM_AVAIL=$(grep MemAvailable /proc/meminfo | awk '{printf "%d", $2/1024}')
    MEM_USED=$((MEM_TOTAL - MEM_AVAIL))
    SWAP_TOTAL=$(grep SwapTotal /proc/meminfo | awk '{printf "%d", $2/1024}')
    SWAP_FREE=$(grep SwapFree /proc/meminfo | awk '{printf "%d", $2/1024}')
    SWAP_USED=$((SWAP_TOTAL - SWAP_FREE))

    # 4. Disk (MB)
    DISK_INFO=$(df -m / | tail -n 1)
    DISK_TOTAL=$(echo $DISK_INFO | awk '{print $2}')
    DISK_USED=$(echo $DISK_INFO | awk '{print $3}')

    # 5. Network (bytes/sec)
    NET_INFO=$(grep "$MAIN_INTERFACE:" /proc/net/dev)
    RX_BYTES=$(echo $NET_INFO | awk '{print $2}')
    TX_BYTES=$(echo $NET_INFO | awk '{print $10}')

    if [ $PREV_RX_BYTES -eq 0 ]; then
        RX_RATE=0
        TX_RATE=0
    else
        RX_RATE=$((RX_BYTES - PREV_RX_BYTES))
        TX_RATE=$((TX_BYTES - PREV_TX_BYTES))
        if [ $RX_RATE -lt 0 ]; then RX_RATE=0; fi
        if [ $TX_RATE -lt 0 ]; then TX_RATE=0; fi
    fi

    PREV_RX_BYTES=$RX_BYTES
    PREV_TX_BYTES=$TX_BYTES

    # 6. Build JSON
    JSON_DATA=$(cat <<EOF
{
  "token": "$TOKEN",
  "host": {
    "name": "$HOSTNAME",
    "os": "$OS_NAME",
    "kernel": "$KERNEL",
    "uptime": $UPTIME
  },
  "cpu": {
    "cores": $CPU_CORES,
    "usage": $CPU_USAGE,
    "model": "$CPU_MODEL",
    "load": [$LOAD_AVG]
  },
  "memory": {
    "total": $MEM_TOTAL,
    "used": $MEM_USED,
    "swap_used": $SWAP_USED
  },
  "disk": {
    "total": $DISK_TOTAL,
    "used": $DISK_USED
  },
  "network": {
    "interface": "$MAIN_INTERFACE",
    "rx_rate": $RX_RATE,
    "tx_rate": $TX_RATE,
    "rx_total": $RX_BYTES,
    "tx_total": $TX_BYTES
  }
}
EOF
)

    # 7. Send
    if [ "$DEBUG" = true ]; then
        echo "$JSON_DATA"
    else
        curl -k -H "Content-Type: application/json" -X POST -d "$JSON_DATA" --connect-timeout 5 -m 5 -s "$SERVER_URL" > /dev/null 2>&1
    fi
}

run_monitor() {
    echo "Monitor Agent started"
    echo "Interface: $MAIN_INTERFACE"
    echo "Interval: ${INTERVAL}s"
    echo "Server: $SERVER_URL"
    echo "---"

    while true; do
        send_report
        sleep $INTERVAL
    done
}

# --- systemd install ---
if [ "$1" = "install" ]; then
    echo "Installing monitor agent..."

    cp "$0" /usr/local/bin/vps-agent.sh
    chmod +x /usr/local/bin/vps-agent.sh

    cat > /etc/systemd/system/vps-agent.service <<'UNIT'
[Unit]
Description=VPS Monitor Agent
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/vps-agent.sh
Restart=always
RestartSec=5s

[Install]
WantedBy=multi-user.target
UNIT

    systemctl daemon-reload
    systemctl enable vps-agent
    systemctl restart vps-agent

    echo "Install complete. Monitor service started."
    exit 0
fi

# --- uninstall ---
if [ "$1" = "uninstall" ]; then
    echo "Uninstalling monitor agent..."
    systemctl stop vps-agent 2>/dev/null
    systemctl disable vps-agent 2>/dev/null
    rm -f /etc/systemd/system/vps-agent.service
    rm -f /usr/local/bin/vps-agent.sh
    systemctl daemon-reload
    echo "Uninstall complete."
    exit 0
fi

run_monitor
