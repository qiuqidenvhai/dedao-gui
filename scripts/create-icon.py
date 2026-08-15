#!/usr/bin/env python3
"""
Create a multi-size ICO file from a PNG source.
This fixes the Windows app icon issue where only one size was included.
"""

import struct
import io
import sys
from pathlib import Path

try:
    from PIL import Image
except ImportError:
    print("PIL not found, installing...")
    import subprocess
    subprocess.check_call([sys.executable, "-m", "pip", "install", "Pillow"])
    from PIL import Image

# Paths
BASE_DIR = Path(r"D:\软件\得到GUI")
PNG_PATH = BASE_DIR / "build" / "appicon.png"
ICO_PATH = BASE_DIR / "build" / "windows" / "icon.ico"

# ICO sizes (standard Windows icon sizes)
SIZES = [16, 32, 48, 64, 128, 256]

def create_icon(source_path: Path, target_path: Path):
    """Create a multi-size ICO file from PNG."""

    print(f"Source: {source_path}")
    print(f"Target: {target_path}")

    if not source_path.exists():
        raise FileNotFoundError(f"PNG file not found: {source_path}")

    # Open source image
    img = Image.open(source_path)
    print(f"Original size: {img.size[0]}x{img.size[1]}")

    # Collect all resized images
    images_data = []
    for size in SIZES:
        # Resize image maintaining aspect ratio
        img_resized = img.resize((size, size), Image.Resampling.LANCZOS)

        # Convert to RGBA for transparency support
        if img_resized.mode != 'RGBA':
            img_resized = img_resized.convert('RGBA')

        # Save to memory as PNG
        buf = io.BytesIO()
        img_resized.save(buf, format='PNG')
        png_data = buf.getvalue()
        buf.close()

        images_data.append({
            'size': size,
            'data': png_data,
            'width': size,
            'height': size
        })
        print(f"  Generated {size}x{size} icon ({len(png_data)} bytes)")

    # Build ICO file
    # ICO header: 6 bytes
    # Directory entries: 16 bytes each
    # Image data follows

    # Calculate offsets
    header_size = 6
    directory_size = len(SIZES) * 16
    data_offset = header_size + directory_size

    # Write to file
    with open(target_path, 'wb') as f:
        # ICONDIR header (6 bytes)
        f.write(struct.pack('<HHH', 0, 1, len(SIZES)))  # Reserved, Type(1=ICO), Count

        # IMAGE_DIRECTORY entries (16 bytes each)
        for item in images_data:
            width = 0 if item['size'] == 256 else item['size']
            height = 0 if item['size'] == 256 else item['size']

            f.write(struct.pack('<BBBBHHII',
                width,      # Width (0 = 256)
                height,     # Height (0 = 256)
                0,          # Color palette
                0,          # Reserved
                1,          # Color planes
                32,         # Bits per pixel
                len(item['data']),  # Image data size
                data_offset       # Data offset
            ))

            data_offset += len(item['data'])

        # Write all image data
        for item in images_data:
            f.write(item['data'])

    img.close()

    # Verify
    ico_size = target_path.stat().st_size
    print(f"\nSuccess! ICO created: {target_path}")
    print(f"File size: {ico_size / 1024:.2f} KB")
    print(f"Included sizes: {', '.join(map(str, SIZES))}")

if __name__ == "__main__":
    try:
        create_icon(PNG_PATH, ICO_PATH)
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)
