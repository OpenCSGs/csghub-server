#!/bin/bash

while true; do
  if test -f "/sys/fs/cgroup/cpu.max"; then
    max_memory=$(cat /sys/fs/cgroup/memory.max)
    current_memory=$(cat /sys/fs/cgroup/memory.current)
  fi

  if test -f "/sys/fs/cgroup/memory/memory.limit_in_bytes"; then
    max_memory=$(cat /sys/fs/cgroup/memory/memory.limit_in_bytes)
    MEMORY_STAT_PATH="/sys/fs/cgroup/memory/memory.stat"
    current_memory=$(awk '$1 == "rss" {print $2}' $MEMORY_STAT_PATH)
  fi
  
  if [ "${max_memory}" == "max" ]; then
    sleep 86400
    continue
  fi
  # reserve 200M
  threshold=209715200
  less_max_memory=$((max_memory - threshold))
  if [ "$current_memory" -gt "$less_max_memory" ]; then
    # Get the PID of the process with the highest memory usage
    pid=$(ps -eo pid,%mem --sort=-%mem | awk 'NR==2 {print $1}')

    # Kill the process
    kill "$pid"
    echo "Process with PID $pid killed due to memory exceeding the limit."
  fi

  sleep 10
done