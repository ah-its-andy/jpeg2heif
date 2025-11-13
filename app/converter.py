"""
Image converter with EXIF metadata preservation
Handles JPEG to HEIC conversion while preserving DateTimeOriginal and other metadata
"""
import os
import logging
import tempfile
import shutil
from datetime import datetime
from typing import Dict, Tuple, Optional
from pathlib import Path

import piexif
from PIL import Image
import pillow_heif

logger = logging.getLogger(__name__)

# Register HEIF opener with Pillow
pillow_heif.register_heif_opener()


class MetadataExtractor:
    """Extract and process EXIF metadata from JPEG files"""
    
    @staticmethod
    def extract_exif(image_path: str) -> Tuple[Optional[bytes], Dict[str, str]]:
        """
        Extract EXIF data from JPEG
        Returns: (exif_bytes, metadata_summary_dict)
        """
        metadata_summary = {}
        exif_bytes = None
        
        try:
            # Load EXIF data
            exif_dict = piexif.load(image_path)
            
            # Extract DateTimeOriginal (priority order)
            datetime_original = None
            if piexif.ExifIFD.DateTimeOriginal in exif_dict.get("Exif", {}):
                datetime_original = exif_dict["Exif"][piexif.ExifIFD.DateTimeOriginal]
                if isinstance(datetime_original, bytes):
                    datetime_original = datetime_original.decode('utf-8', errors='ignore')
                metadata_summary['DateTimeOriginal'] = datetime_original
            elif piexif.ImageIFD.DateTime in exif_dict.get("0th", {}):
                datetime_original = exif_dict["0th"][piexif.ImageIFD.DateTime]
                if isinstance(datetime_original, bytes):
                    datetime_original = datetime_original.decode('utf-8', errors='ignore')
                metadata_summary['DateTime'] = datetime_original
            
            # Extract other useful fields
            if "0th" in exif_dict:
                # Camera make and model
                if piexif.ImageIFD.Make in exif_dict["0th"]:
                    make = exif_dict["0th"][piexif.ImageIFD.Make]
                    if isinstance(make, bytes):
                        make = make.decode('utf-8', errors='ignore')
                    metadata_summary['Make'] = make
                
                if piexif.ImageIFD.Model in exif_dict["0th"]:
                    model = exif_dict["0th"][piexif.ImageIFD.Model]
                    if isinstance(model, bytes):
                        model = model.decode('utf-8', errors='ignore')
                    metadata_summary['Model'] = model
            
            # Extract GPS data
            if "GPS" in exif_dict and exif_dict["GPS"]:
                has_gps = any(exif_dict["GPS"].values())
                if has_gps:
                    metadata_summary['GPS'] = 'present'
                    
                    # Try to extract coordinates
                    if piexif.GPSIFD.GPSLatitude in exif_dict["GPS"]:
                        metadata_summary['GPSLatitude'] = 'present'
                    if piexif.GPSIFD.GPSLongitude in exif_dict["GPS"]:
                        metadata_summary['GPSLongitude'] = 'present'
            
            # Extract lens info if available
            if "Exif" in exif_dict:
                if piexif.ExifIFD.LensModel in exif_dict["Exif"]:
                    lens = exif_dict["Exif"][piexif.ExifIFD.LensModel]
                    if isinstance(lens, bytes):
                        lens = lens.decode('utf-8', errors='ignore')
                    metadata_summary['LensModel'] = lens
            
            # Dump EXIF to bytes for embedding
            exif_bytes = piexif.dump(exif_dict)
            
            logger.debug(f"Extracted EXIF from {image_path}: {metadata_summary}")
            
        except Exception as e:
            logger.warning(f"Failed to extract EXIF from {image_path}: {e}")
        
        return exif_bytes, metadata_summary
    
    @staticmethod
    def extract_datetime(metadata_summary: Dict[str, str]) -> Optional[str]:
        """Extract the primary datetime from metadata summary"""
        return metadata_summary.get('DateTimeOriginal') or metadata_summary.get('DateTime')


class ImageConverter:
    """Convert JPEG images to HEIC with metadata preservation"""
    
    def __init__(self, quality: int = 90, preserve_metadata: bool = True):
        self.quality = quality
        self.preserve_metadata = preserve_metadata
    
    def convert(self, source_path: str, target_path: str) -> Dict[str, any]:
        """
        Convert JPEG to HEIC with metadata preservation
        
        Returns dict with:
        - success: bool
        - error: str (if failed)
        - metadata_preserved: bool
        - metadata_summary: str
        - source_datetime: str
        - target_datetime: str
        """
        result = {
            'success': False,
            'error': None,
            'metadata_preserved': False,
            'metadata_summary': '',
            'source_datetime': None,
            'target_datetime': None
        }
        
        try:
            # Validate source file
            if not os.path.exists(source_path):
                raise FileNotFoundError(f"Source file not found: {source_path}")
            
            # Create target directory if needed
            target_dir = os.path.dirname(target_path)
            if target_dir and not os.path.exists(target_dir):
                os.makedirs(target_dir, exist_ok=True)
            
            # Extract metadata from source
            exif_bytes = None
            metadata_summary = {}
            source_datetime = None
            
            if self.preserve_metadata:
                exif_bytes, metadata_summary = MetadataExtractor.extract_exif(source_path)
                source_datetime = MetadataExtractor.extract_datetime(metadata_summary)
                result['source_datetime'] = source_datetime
            
            # Open and convert image
            with Image.open(source_path) as img:
                # Ensure RGB mode for HEIC
                if img.mode not in ('RGB', 'RGBA'):
                    img = img.convert('RGB')
                
                # Write to temporary file first (atomic operation)
                temp_fd, temp_path = tempfile.mkstemp(suffix='.heic', dir=target_dir)
                os.close(temp_fd)
                
                try:
                    # Save as HEIC with EXIF
                    save_kwargs = {
                        'format': 'HEIF',
                        'quality': self.quality,
                    }
                    
                    # Add EXIF if available
                    if exif_bytes:
                        save_kwargs['exif'] = exif_bytes
                    
                    img.save(temp_path, **save_kwargs)
                    
                    # Verify the file was created
                    if not os.path.exists(temp_path) or os.path.getsize(temp_path) == 0:
                        raise Exception("Failed to create HEIC file")
                    
                    # Move to final destination atomically
                    shutil.move(temp_path, target_path)
                    
                    # Verify metadata preservation
                    if self.preserve_metadata and source_datetime:
                        target_datetime = self._verify_datetime(target_path)
                        result['target_datetime'] = target_datetime
                        
                        # Check if datetime matches
                        if target_datetime and target_datetime == source_datetime:
                            result['metadata_preserved'] = True
                            metadata_summary['verification'] = 'DateTimeOriginal verified'
                        elif target_datetime:
                            metadata_summary['verification'] = f'DateTimeOriginal mismatch: {source_datetime} != {target_datetime}'
                        else:
                            metadata_summary['verification'] = 'DateTimeOriginal not found in target'
                    elif self.preserve_metadata:
                        result['metadata_preserved'] = True
                        metadata_summary['verification'] = 'No source datetime to verify'
                    
                    # Build metadata summary string
                    summary_parts = []
                    for key, value in metadata_summary.items():
                        summary_parts.append(f"{key}: {value}")
                    result['metadata_summary'] = '; '.join(summary_parts)
                    
                    result['success'] = True
                    logger.info(f"Converted {source_path} -> {target_path}")
                    
                except Exception as e:
                    # Clean up temp file on error
                    if os.path.exists(temp_path):
                        os.remove(temp_path)
                    raise
        
        except Exception as e:
            result['error'] = str(e)
            logger.error(f"Conversion failed for {source_path}: {e}")
        
        return result
    
    def _verify_datetime(self, heic_path: str) -> Optional[str]:
        """Verify DateTimeOriginal in HEIC file"""
        try:
            exif_dict = piexif.load(heic_path)
            
            if piexif.ExifIFD.DateTimeOriginal in exif_dict.get("Exif", {}):
                datetime_original = exif_dict["Exif"][piexif.ExifIFD.DateTimeOriginal]
                if isinstance(datetime_original, bytes):
                    datetime_original = datetime_original.decode('utf-8', errors='ignore')
                return datetime_original
            elif piexif.ImageIFD.DateTime in exif_dict.get("0th", {}):
                datetime_value = exif_dict["0th"][piexif.ImageIFD.DateTime]
                if isinstance(datetime_value, bytes):
                    datetime_value = datetime_value.decode('utf-8', errors='ignore')
                return datetime_value
        except Exception as e:
            logger.warning(f"Failed to verify datetime in {heic_path}: {e}")
        
        return None


def get_heic_target_path(jpeg_path: str) -> str:
    """
    Calculate target HEIC path based on JPEG path
    Example: /a/b/c/pic.jpg -> /a/b/heic/pic.heic
    """
    jpeg_path = os.path.abspath(jpeg_path)
    parent_dir = os.path.dirname(jpeg_path)
    grandparent_dir = os.path.dirname(parent_dir)
    basename = os.path.basename(jpeg_path)
    name_without_ext = os.path.splitext(basename)[0]
    
    heic_dir = os.path.join(grandparent_dir, 'heic')
    heic_path = os.path.join(heic_dir, f"{name_without_ext}.heic")
    
    # Handle existing files by adding index
    if os.path.exists(heic_path):
        index = 1
        while True:
            indexed_path = os.path.join(heic_dir, f"{name_without_ext}_{index}.heic")
            if not os.path.exists(indexed_path):
                heic_path = indexed_path
                break
            index += 1
    
    return heic_path
