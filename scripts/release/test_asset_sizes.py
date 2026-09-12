import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("asset_sizes", Path(__file__).with_name("check-asset-sizes.py"))
asset_sizes = importlib.util.module_from_spec(spec)
spec.loader.exec_module(asset_sizes)


class AssetSizeTests(unittest.TestCase):
    def test_github_limit_boundary(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            asset = root / "rootfs.ext4.zst"
            for size, accepted in [(2**31 - 1, True), (2**31, False), (2**31 + 1, False)]:
                with self.subTest(size=size):
                    with asset.open("wb") as stream:
                        stream.truncate(size)
                    if accepted:
                        asset_sizes.check_sizes(root)
                    else:
                        with self.assertRaisesRegex(ValueError, "rootfs.ext4.zst"):
                            asset_sizes.check_sizes(root)

    def test_empty_directory_is_rejected(self):
        with tempfile.TemporaryDirectory() as temp:
            with self.assertRaisesRegex(ValueError, "empty"):
                asset_sizes.check_sizes(Path(temp))
