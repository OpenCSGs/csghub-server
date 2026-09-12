#!/bin/bash

# Detect AMD GPU (ROCm) and configure environment for multi-node Ray cluster
if [ -e /dev/kfd ] || command -v rocm-smi &>/dev/null; then
    echo "AMD GPU detected, configuring ROCm multi-node Ray environment"
    # HIP_VISIBLE_DEVICES controls which AMD GPUs are visible to the runtime
    # (analogous to CUDA_VISIBLE_DEVICES for NVIDIA). Build the device list
    # from GPU_NUM so only the assigned GPUs are exposed.
    if [ -z "${HIP_VISIBLE_DEVICES:-}" ] && [ -n "${GPU_NUM:-}" ]; then
        HIP_DEVICES=$(python3 -c "print(','.join(str(i) for i in range($GPU_NUM)))")
        export HIP_VISIBLE_DEVICES="$HIP_DEVICES"
    fi
    # Unset ROCR_VISIBLE_DEVICES: the runner sets it to "none" on non-AMD
    # nodes to hide AMD GPUs. On AMD nodes we must clear it so the devices
    # remain visible for multi-node RCCL communication.
    unset ROCR_VISIBLE_DEVICES
    # Enable P2P for multi-node; the Dockerfile sets NCCL_P2P_DISABLE=1 for
    # single-node safety, but multi-node RCCL needs P2P enabled.
    export NCCL_P2P_DISABLE=0
fi

subcommand=$1
shift

ray_port=6379
ray_init_timeout=300
declare -a start_params

case "$subcommand" in
  worker)
    ray_address=""
    while [ $# -gt 0 ]; do
      case "$1" in
        --ray_address=*)
          ray_address="${1#*=}"
          ;;
        --ray_port=*)
          ray_port="${1#*=}"
          ;;
        --ray_init_timeout=*)
          ray_init_timeout="${1#*=}"
          ;;
        *)
          start_params+=("$1")
      esac
      shift
    done

    if [ -z "$ray_address" ]; then
      echo "Error: Missing argument --ray_address"
      exit 1
    fi

    for (( i=0; i < $ray_init_timeout; i+=5 )); do
      ray start --address=$ray_address:$ray_port --block "${start_params[@]}"
      if [ $? -eq 0 ]; then
        echo "Worker: Ray runtime started with head address $ray_address:$ray_port"
        exit 0
      fi
      echo "Waiting until the ray worker is active..."
      sleep 5s;
    done
    echo "Ray worker starts timeout, head address: $ray_address:$ray_port"
    exit 1
    ;;

  leader)
    ray_cluster_size=""
    while [ $# -gt 0 ]; do
          case "$1" in
            --ray_port=*)
              ray_port="${1#*=}"
              ;;
            --ray_cluster_size=*)
              ray_cluster_size="${1#*=}"
              ;;
            --ray_init_timeout=*)
              ray_init_timeout="${1#*=}"
              ;;
            *)
              start_params+=("$1")
          esac
          shift
    done

    if [ -z "$ray_cluster_size" ]; then
      echo "Error: Missing argument --ray_cluster_size"
      exit 1
    fi

    # start the ray daemon
    ray start --head --port=$ray_port "${start_params[@]}"

    # wait until all workers are active
    for (( i=0; i < $ray_init_timeout; i+=5 )); do
        active_nodes=`python3 -c 'import ray; ray.init(); print(sum(node["Alive"] for node in ray.nodes()))'`
        if [ $active_nodes -eq $ray_cluster_size ]; then
          echo "All ray workers are active and the ray cluster is initialized successfully."
          exit 0
        fi
        echo "Wait for all ray workers to be active. $active_nodes/$ray_cluster_size is active"
        sleep 5s;
    done

    echo "Waiting for all ray workers to be active timed out."
    exit 1
    ;;

  *)
    echo "unknown subcommand: $subcommand"
    exit 1
    ;;
esac