#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"

python3.10 /etc/csghub/entry.py
python3.11 /etc/csghub/start.py