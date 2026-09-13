#!/usr/bin/env python3
"""Prepare only the pinned renderer for a small Windows engineering build."""
import hashlib
import json
from pathlib import Path
import subprocess
import sys

recipe = Path(__file__).resolve().parent
lock = json.loads((recipe / 'sources.lock.json').read_text())['virglrenderer']
source = Path(sys.argv[1]).resolve()
source.mkdir(parents=True, exist_ok=False)

def git(*args):
    subprocess.run(['git', '-C', str(source), *args], check=True)

git('init', '--quiet')
git('remote', 'add', 'origin', lock['repository'])
git('fetch', '--quiet', '--depth=256', 'origin', lock['commit'], lock['baseCommit'])
git('checkout', '--quiet', '--detach', lock['commit'])
git('merge-base', '--is-ancestor', lock['baseCommit'], lock['commit'])
for patch in lock.get('patches', []):
    path = recipe / patch['file']
    if hashlib.sha256(path.read_bytes()).hexdigest() != patch['sha256']:
        raise SystemExit(f'Patch digest mismatch: {path}')
    git('apply', '--index', str(path))
