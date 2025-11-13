#!/usr/bin/env python3
"""
Create sample JPEG files with EXIF metadata for testing
"""
import os
import sys
from datetime import datetime, timedelta
from PIL import Image
import piexif

def create_sample_jpeg(output_path: str, size=(800, 600), color='blue', 
                       datetime_str=None, with_gps=False):
    """Create a sample JPEG with EXIF metadata"""
    
    # Create image
    img = Image.new('RGB', size, color=color)
    
    # Create EXIF data
    exif_dict = {
        "0th": {
            piexif.ImageIFD.Make: b"TestCamera",
            piexif.ImageIFD.Model: b"TestModel Pro",
            piexif.ImageIFD.Software: b"Test Generator v1.0",
            piexif.ImageIFD.DateTime: (datetime_str or datetime.now().strftime("%Y:%m:%d %H:%M:%S")).encode('utf-8')
        },
        "Exif": {
            piexif.ExifIFD.DateTimeOriginal: (datetime_str or datetime.now().strftime("%Y:%m:%d %H:%M:%S")).encode('utf-8'),
            piexif.ExifIFD.LensModel: b"Test Lens 50mm f/1.8",
            piexif.ExifIFD.FocalLength: (50, 1),
            piexif.ExifIFD.FNumber: (18, 10),
            piexif.ExifIFD.ISOSpeedRatings: 400
        }
    }
    
    # Add GPS if requested
    if with_gps:
        exif_dict["GPS"] = {
            piexif.GPSIFD.GPSLatitude: ((40, 1), (44, 1), (54, 1)),
            piexif.GPSIFD.GPSLatitudeRef: b'N',
            piexif.GPSIFD.GPSLongitude: ((73, 1), (59, 1), (10, 1)),
            piexif.GPSIFD.GPSLongitudeRef: b'W',
            piexif.GPSIFD.GPSAltitude: (100, 1),
            piexif.GPSIFD.GPSAltitudeRef: 0
        }
    
    # Dump and save
    exif_bytes = piexif.dump(exif_dict)
    img.save(output_path, "JPEG", exif=exif_bytes, quality=95)
    print(f"Created: {output_path}")


def main():
    """Generate sample test images"""
    
    # Create output directory
    output_dir = "data/images/test_samples"
    os.makedirs(output_dir, exist_ok=True)
    
    print("Generating sample JPEG files...")
    print(f"Output directory: {output_dir}")
    print()
    
    # Sample 1: Basic image with datetime
    create_sample_jpeg(
        os.path.join(output_dir, "sample_1_basic.jpg"),
        color='red',
        datetime_str="2024:01:15 14:30:00"
    )
    
    # Sample 2: Image with GPS
    create_sample_jpeg(
        os.path.join(output_dir, "sample_2_with_gps.jpg"),
        color='green',
        datetime_str="2024:02:20 10:15:30",
        with_gps=True
    )
    
    # Sample 3: Different date
    create_sample_jpeg(
        os.path.join(output_dir, "sample_3_different_date.jpg"),
        color='blue',
        datetime_str="2023:12:25 18:00:00"
    )
    
    # Sample 4: Recent date
    recent_date = (datetime.now() - timedelta(hours=2)).strftime("%Y:%m:%d %H:%M:%S")
    create_sample_jpeg(
        os.path.join(output_dir, "sample_4_recent.jpg"),
        color='yellow',
        datetime_str=recent_date
    )
    
    # Sample 5: Large image
    create_sample_jpeg(
        os.path.join(output_dir, "sample_5_large.jpg"),
        size=(3000, 2000),
        color='purple',
        datetime_str="2024:03:10 16:45:22",
        with_gps=True
    )
    
    print()
    print(f"✓ Generated 5 sample JPEG files in {output_dir}")
    print()
    print("To test conversion:")
    print(f"  1. Set WATCH_DIRS={output_dir} in .env")
    print("  2. Run: python -m app.main")
    print("  3. Check output in data/heic/")
    print()


if __name__ == '__main__':
    try:
        main()
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)
