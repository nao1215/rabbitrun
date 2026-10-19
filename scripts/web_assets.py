#!/usr/bin/env python3
"""web_assets.py - make the smaller copy of assets/ that the browser build embeds.

    python3 scripts/web_assets.py [SRC] [DST]     # defaults: assets webassets (make web-assets)

The desktop game embeds assets/ as it is, about 210 MB, most of it the characters'
640x1600 PNG portraits. A browser downloads the whole WebAssembly file before the game
starts, so the browser build (assets_js.go) embeds DST instead, which this script writes:

- The characters' pictures become lossy WebP (with alpha for the portraits), no larger
  than the game ever draws them on its 720x900 screen: a portrait is decoded at most 800
  pixels high (character.standingMaxH), so it is stored at that height; an illustration is
  stored at the size that fits the screen, or covers it when it is nearly the screen's
  shape (the endings, drawn to cover the screen). The game reads .webp after .jpg and .png.
- The artwork of the screens (ui/) becomes WebP at its own size: it is drawn at about
  that size already.
- Everything else the game embeds (game.json, blocks/, fonts/) is copied as it is: the
  blocks are read by their .png names, and are small.

DST is generated output and is not committed. A picture already converted from an
unchanged source is kept, and files whose source is gone are removed, so a second run
only converts what changed (make clean removes DST, to convert everything again after
changing the settings below).
It needs Pillow built with WebP support (python3-pil on Debian and Ubuntu).
"""

import os
import shutil
import sys
from concurrent.futures import ProcessPoolExecutor

from PIL import Image, features

SCREEN_W, SCREEN_H = 720, 900
PORTRAIT_MAX_H = 800  # character.standingMaxH
PORTRAIT_QUALITY = 75
ILLUSTRATION_QUALITY = 75
UI_QUALITY = 80
COPIED_DIRS = ("blocks", "fonts")


def illustration_scale(w, h):
    """The scale an illustration is stored at (at most 1)."""
    fit = min(SCREEN_W / w, SCREEN_H / h)
    cover = max(SCREEN_W / w, SCREEN_H / h)
    # Nearly the screen's shape (the endings): drawn to cover the screen. A tall 1:2
    # illustration is drawn to fit it (the gallery, the clear) or smaller (the road).
    return min(1.0, cover if cover / fit < 1.25 else fit)


def convert(job):
    src, dst, kind = job
    if os.path.exists(dst) and os.path.getmtime(dst) >= os.path.getmtime(src):
        return os.path.getsize(dst)
    im = Image.open(src)
    w, h = im.size
    if kind == "ui":
        im = im.convert("RGBA" if src.endswith(".png") else "RGB")
        scale, quality = 1.0, UI_QUALITY
    elif src.endswith(".png"):
        im = im.convert("RGBA")
        scale, quality = min(1.0, PORTRAIT_MAX_H / h), PORTRAIT_QUALITY
    else:
        im = im.convert("RGB")
        scale, quality = illustration_scale(w, h), ILLUSTRATION_QUALITY
    if scale < 1:
        im = im.resize((round(w * scale), round(h * scale)), Image.LANCZOS)
    im.save(dst, "WEBP", quality=quality, method=6, alpha_quality=100)
    return os.path.getsize(dst)


def main():
    src = sys.argv[1] if len(sys.argv) > 1 else "assets"
    dst = sys.argv[2] if len(sys.argv) > 2 else "webassets"
    if not features.check("webp"):
        sys.exit("web_assets.py: this Pillow cannot write WebP (install python3-pil or pillow with libwebp)")
    if os.path.abspath(dst) == os.path.abspath(src):
        sys.exit("web_assets.py: DST must not be SRC")
    keep = set()
    for d in COPIED_DIRS:
        for root, _, names in os.walk(os.path.join(src, d)):
            for name in names:
                out = os.path.join(dst, os.path.relpath(os.path.join(root, name), src))
                os.makedirs(os.path.dirname(out), exist_ok=True)
                shutil.copy2(os.path.join(root, name), out)
                keep.add(out)
    jobs = []
    os.makedirs(os.path.join(dst, "ui"), exist_ok=True)
    for name in sorted(os.listdir(os.path.join(src, "ui"))):
        stem, ext = os.path.splitext(name)
        if ext in (".png", ".jpg"):
            jobs.append((os.path.join(src, "ui", name), os.path.join(dst, "ui", stem + ".webp"), "ui"))
    chars = os.path.join(src, "characters")
    for cid in sorted(os.listdir(chars)):
        manifest = os.path.join(chars, cid, "game.json")
        if not os.path.isfile(manifest):
            continue
        out = os.path.join(dst, "characters", cid)
        os.makedirs(os.path.join(out, "images"), exist_ok=True)
        shutil.copy2(manifest, out)
        keep.add(os.path.join(out, "game.json"))
        images = os.path.join(chars, cid, "images")
        for name in sorted(os.listdir(images)):
            stem, ext = os.path.splitext(name)
            if ext in (".png", ".jpg"):
                jobs.append((os.path.join(images, name), os.path.join(out, "images", stem + ".webp"), "character"))
    keep.update(j[1] for j in jobs)
    for root, _, names in os.walk(dst):
        for name in names:
            if os.path.join(root, name) not in keep:
                os.remove(os.path.join(root, name))
    with ProcessPoolExecutor() as pool:
        total = sum(pool.map(convert, jobs, chunksize=4))
    print(f"web_assets.py: {len(jobs)} pictures in {total / 1e6:.1f} MB under {dst}/")


if __name__ == "__main__":
    main()
