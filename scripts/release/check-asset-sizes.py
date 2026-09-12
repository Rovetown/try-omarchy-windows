#!/usr/bin/env python3
"""Reject assets GitHub Releases cannot accept before creating a draft."""

from pathlib import Path
import sys

MAX_ASSET_BYTES = 2**31


def check_sizes(directory: Path) -> None:
    assets = list(directory.iterdir())
    if not assets:
        raise ValueError("release asset directory is empty")
    for asset in assets:
        if not asset.is_file():
            raise ValueError(f"release asset is not a file: {asset.name}")
        size = asset.stat().st_size
        if size >= MAX_ASSET_BYTES:
            raise ValueError(
                f"{asset.name}: {size} bytes exceeds GitHub's asset limit; "
                f"each asset must be smaller than {MAX_ASSET_BYTES} bytes"
            )


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("Usage: check-asset-sizes.py ARTIFACT_DIR")
    try:
        check_sizes(Path(sys.argv[1]))
    except (OSError, ValueError) as error:
        raise SystemExit(str(error))
