#!/usr/bin/env python3
"""生成 ctfile-down 的 macOS 图标 (.icns)。

绘制一个圆角渐变方块 + 白色下载箭头，输出 iconset 后用 iconutil 打包。
"""
import os
import shutil
import subprocess
import sys
import tempfile

from PIL import Image, ImageDraw

S = 1024
TOP = (59, 130, 246)    # #3b82f6
BOTTOM = (124, 58, 237)  # #7c3aed
PAD = 88
RADIUS = 190


def lerp(a, b, t):
    return tuple(round(a[i] + (b[i] - a[i]) * t) for i in range(3))


def render() -> Image.Image:
    # 竖向渐变
    grad = Image.new("RGB", (S, S))
    d = ImageDraw.Draw(grad)
    for y in range(S):
        d.line([(0, y), (S, y)], fill=lerp(TOP, BOTTOM, y / (S - 1)))

    # 圆角遮罩
    mask = Image.new("L", (S, S), 0)
    ImageDraw.Draw(mask).rounded_rectangle(
        [PAD, PAD, S - PAD, S - PAD], radius=RADIUS, fill=255
    )

    base = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    base.paste(grad, (0, 0), mask)

    # 白色下载箭头
    fg = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    fd = ImageDraw.Draw(fg)
    cx = S // 2
    fd.rounded_rectangle([cx - 54, 286, cx + 54, 596], radius=18, fill=(255, 255, 255, 255))
    fd.polygon([(cx - 158, 556), (cx + 158, 556), (cx, 762)], fill=(255, 255, 255, 255))
    fd.rounded_rectangle([cx - 176, 800, cx + 176, 866], radius=33, fill=(255, 255, 255, 255))

    base.alpha_composite(fg)
    return base


def main():
    out = sys.argv[1] if len(sys.argv) > 1 else "macos/icon.icns"
    img = render()

    iconset = tempfile.mkdtemp(suffix=".iconset")
    try:
        specs = [
            (16, "icon_16x16.png"), (32, "icon_16x16@2x.png"),
            (32, "icon_32x32.png"), (64, "icon_32x32@2x.png"),
            (128, "icon_128x128.png"), (256, "icon_128x128@2x.png"),
            (256, "icon_256x256.png"), (512, "icon_256x256@2x.png"),
            (512, "icon_512x512.png"), (1024, "icon_512x512@2x.png"),
        ]
        for size, name in specs:
            img.resize((size, size), Image.LANCZOS).save(os.path.join(iconset, name))
        os.makedirs(os.path.dirname(os.path.abspath(out)), exist_ok=True)
        subprocess.run(["iconutil", "-c", "icns", iconset, "-o", out], check=True)
        print(f"icon -> {out}")
    finally:
        shutil.rmtree(iconset, ignore_errors=True)


if __name__ == "__main__":
    main()
