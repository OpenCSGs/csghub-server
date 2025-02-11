#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"

python3 /etc/csghub/entry.py

text-embeddings-router --model-id "/workspace/$REPO_ID" --port 8000