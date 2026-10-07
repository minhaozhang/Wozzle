"""One-off generator for Wozzle's tray/window icon (W glyph, dark rounded square)."""
from PIL import Image, ImageDraw

S = 1024
BG = (11, 15, 20, 255)
BLUE = (79, 140, 255, 255)

img = Image.new("RGBA", (S, S), (0, 0, 0, 0))
d = ImageDraw.Draw(img)
d.rounded_rectangle([64, 64, S - 64, S - 64], radius=210, fill=BG)

lw = 128
pts = [(270, 330), (392, 700), (512, 430), (632, 700), (754, 330)]
d.line(pts, fill=BLUE, width=lw, joint="curve")
for p in (pts[0], pts[-1]):
    d.ellipse([p[0] - lw / 2, p[1] - lw / 2, p[0] + lw / 2, p[1] + lw / 2], fill=BLUE)

img.save("internal/desktop/wozzle.ico",
         sizes=[(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)])
img.save("docs/wozzle-icon.png")
print("icon written")
