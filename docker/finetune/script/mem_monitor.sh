#!/bin/bash

while true; do
  max_memory=$(cat /sys/fs/cgroup/memory.max)
  if [ "${max_memory}" == "max" ]; then
    sleep 86400
    continue
  fi
  current_memory=$(cat /sys/fs/cgroup/memory.current)
  threshold=1048576000
  less_max_memory=$((max_memory - threshold))
  if [ "$current_memory" -gt "$less_max_memory" ]; then
    # Get the PID of the process with the highest memory usage
    pid=$(ps -eo pid,%mem --sort=-%mem | awk 'NR==2 {print $1}')

    # Kill the process
    kill "$pid"
    echo "Process with PID $pid killed due to memory exceeding the limit."
  fi

  sleep 6
done