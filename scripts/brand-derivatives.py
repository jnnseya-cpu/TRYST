"""Scaled copies of the supplied logo for the web (docs/spec/01_Product.md §3.1).

The logo itself stays byte-for-byte as supplied (shared/brand/tryst-logo.png, checked by
SHA-256 in each app's sync-brand script). These are the same image resized with Lanczos
resampling, nothing else changed, so pages and app icons don't download the 1.1 MB original:
    python3 scripts/brand-derivatives.py      (needs Pillow; output is committed)
"""
from pathlib import Path

from PIL import Image

brand = Path(__file__).resolve().parent.parent / "shared" / "brand"
out = brand / "web"
out.mkdir(exist_ok=True)
logo = Image.open(brand / "tryst-logo.png").convert("RGBA")
for px in (96, 180, 192, 512):
    logo.resize((px, px), Image.LANCZOS).save(out / f"tryst-logo-{px}.png", optimize=True)
logo.resize((512, 512), Image.LANCZOS).save(out / "tryst-logo-512.webp", quality=88, method=6)
for f in sorted(out.iterdir()):
    print(f.name, f.stat().st_size)
