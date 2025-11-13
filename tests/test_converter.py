"""
Test suite for JPEG to HEIC converter
"""
import os
import sys
import tempfile
import shutil
from pathlib import Path

# Add app to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..'))

import pytest
from PIL import Image
import piexif

from app.converter import ImageConverter, MetadataExtractor, get_heic_target_path
from app.database import Database


class TestMetadataExtraction:
    """Test metadata extraction from JPEG files"""
    
    def create_test_jpeg_with_exif(self, filepath: str, datetime_original: str = "2024:01:15 14:30:00"):
        """Create a test JPEG with EXIF data"""
        # Create a simple test image
        img = Image.new('RGB', (100, 100), color='red')
        
        # Create EXIF data
        exif_dict = {
            "0th": {
                piexif.ImageIFD.Make: b"TestCamera",
                piexif.ImageIFD.Model: b"TestModel",
                piexif.ImageIFD.DateTime: datetime_original.encode('utf-8')
            },
            "Exif": {
                piexif.ExifIFD.DateTimeOriginal: datetime_original.encode('utf-8'),
                piexif.ExifIFD.LensModel: b"TestLens"
            },
            "GPS": {
                piexif.GPSIFD.GPSLatitude: ((40, 1), (44, 1), (54, 1)),
                piexif.GPSIFD.GPSLongitude: ((73, 1), (59, 1), (10, 1))
            }
        }
        
        exif_bytes = piexif.dump(exif_dict)
        img.save(filepath, "JPEG", exif=exif_bytes, quality=95)
        
        return filepath
    
    def test_extract_exif_with_datetime(self):
        """Test extracting EXIF with DateTimeOriginal"""
        with tempfile.TemporaryDirectory() as tmpdir:
            jpeg_path = os.path.join(tmpdir, "test.jpg")
            test_datetime = "2024:01:15 14:30:00"
            
            self.create_test_jpeg_with_exif(jpeg_path, test_datetime)
            
            exif_bytes, metadata = MetadataExtractor.extract_exif(jpeg_path)
            
            assert exif_bytes is not None
            assert 'DateTimeOriginal' in metadata
            assert metadata['DateTimeOriginal'] == test_datetime
            assert 'Make' in metadata
            assert metadata['Make'] == 'TestCamera'
            assert 'GPS' in metadata
    
    def test_extract_datetime(self):
        """Test datetime extraction from metadata summary"""
        metadata = {'DateTimeOriginal': '2024:01:15 14:30:00'}
        datetime = MetadataExtractor.extract_datetime(metadata)
        assert datetime == '2024:01:15 14:30:00'
        
        # Test fallback to DateTime
        metadata2 = {'DateTime': '2024:01:15 14:30:00'}
        datetime2 = MetadataExtractor.extract_datetime(metadata2)
        assert datetime2 == '2024:01:15 14:30:00'


class TestImageConversion:
    """Test image conversion functionality"""
    
    def create_test_jpeg_with_exif(self, filepath: str, datetime_original: str = "2024:01:15 14:30:00"):
        """Create a test JPEG with EXIF data"""
        img = Image.new('RGB', (100, 100), color='blue')
        
        exif_dict = {
            "0th": {
                piexif.ImageIFD.Make: b"TestCamera",
                piexif.ImageIFD.Model: b"TestModel"
            },
            "Exif": {
                piexif.ExifIFD.DateTimeOriginal: datetime_original.encode('utf-8')
            }
        }
        
        exif_bytes = piexif.dump(exif_dict)
        img.save(filepath, "JPEG", exif=exif_bytes, quality=95)
        
        return filepath
    
    def test_conversion_basic(self):
        """Test basic JPEG to HEIC conversion"""
        with tempfile.TemporaryDirectory() as tmpdir:
            jpeg_path = os.path.join(tmpdir, "test.jpg")
            heic_path = os.path.join(tmpdir, "test.heic")
            
            self.create_test_jpeg_with_exif(jpeg_path)
            
            converter = ImageConverter(quality=90, preserve_metadata=False)
            result = converter.convert(jpeg_path, heic_path)
            
            assert result['success'] is True
            assert result['error'] is None
            assert os.path.exists(heic_path)
            assert os.path.getsize(heic_path) > 0
    
    def test_conversion_with_metadata_preservation(self):
        """Test JPEG to HEIC conversion with metadata preservation (CRITICAL TEST)"""
        with tempfile.TemporaryDirectory() as tmpdir:
            jpeg_path = os.path.join(tmpdir, "test_meta.jpg")
            heic_path = os.path.join(tmpdir, "test_meta.heic")
            
            test_datetime = "2024:01:15 14:30:00"
            self.create_test_jpeg_with_exif(jpeg_path, test_datetime)
            
            converter = ImageConverter(quality=90, preserve_metadata=True)
            result = converter.convert(jpeg_path, heic_path)
            
            # Check conversion success
            assert result['success'] is True, f"Conversion failed: {result.get('error')}"
            assert os.path.exists(heic_path)
            
            # Verify metadata preservation
            assert result['source_datetime'] == test_datetime
            assert result['target_datetime'] is not None
            
            # CRITICAL: Verify DateTimeOriginal matches
            assert result['source_datetime'] == result['target_datetime'], \
                f"DateTimeOriginal mismatch: source={result['source_datetime']}, target={result['target_datetime']}"
            
            assert result['metadata_preserved'] is True
            assert 'DateTimeOriginal' in result['metadata_summary']
    
    def test_get_heic_target_path(self):
        """Test target path calculation"""
        # Test: /a/b/c/pic.jpg -> /a/b/heic/pic.heic
        with tempfile.TemporaryDirectory() as tmpdir:
            source_dir = os.path.join(tmpdir, "a", "b", "c")
            os.makedirs(source_dir, exist_ok=True)
            
            source_path = os.path.join(source_dir, "pic.jpg")
            # Create dummy file
            Path(source_path).touch()
            
            target_path = get_heic_target_path(source_path)
            
            expected_dir = os.path.join(tmpdir, "a", "b", "heic")
            expected_path = os.path.join(expected_dir, "pic.heic")
            
            assert target_path == expected_path
    
    def test_get_heic_target_path_with_conflict(self):
        """Test target path calculation with existing file"""
        with tempfile.TemporaryDirectory() as tmpdir:
            source_dir = os.path.join(tmpdir, "a", "b", "c")
            heic_dir = os.path.join(tmpdir, "a", "b", "heic")
            os.makedirs(source_dir, exist_ok=True)
            os.makedirs(heic_dir, exist_ok=True)
            
            source_path = os.path.join(source_dir, "pic.jpg")
            Path(source_path).touch()
            
            # Create existing HEIC file
            existing_heic = os.path.join(heic_dir, "pic.heic")
            Path(existing_heic).touch()
            
            target_path = get_heic_target_path(source_path)
            
            # Should get pic_1.heic
            expected_path = os.path.join(heic_dir, "pic_1.heic")
            assert target_path == expected_path


class TestDatabase:
    """Test database operations"""
    
    def test_database_initialization(self):
        """Test database creation and initialization"""
        with tempfile.TemporaryDirectory() as tmpdir:
            db_path = os.path.join(tmpdir, "test.db")
            db = Database(db_path)
            
            assert os.path.exists(db_path)
            assert db.engine is not None
    
    def test_create_and_get_task(self):
        """Test task creation and retrieval"""
        with tempfile.TemporaryDirectory() as tmpdir:
            db_path = os.path.join(tmpdir, "test.db")
            db = Database(db_path)
            
            task = db.create_task(task_type='once', source_path='/test/image.jpg')
            
            assert task.id is not None
            assert task.task_type == 'once'
            assert task.source_path == '/test/image.jpg'
            assert task.status == 'pending'
            
            # Retrieve task
            retrieved = db.get_task(task.id)
            assert retrieved is not None
            assert retrieved.id == task.id
    
    def test_update_task(self):
        """Test task update"""
        with tempfile.TemporaryDirectory() as tmpdir:
            db_path = os.path.join(tmpdir, "test.db")
            db = Database(db_path)
            
            task = db.create_task(task_type='once', source_path='/test/image.jpg')
            
            db.update_task(
                task.id,
                status='success',
                target_path='/test/output.heic',
                metadata_preserved=True,
                source_datetime='2024:01:15 14:30:00',
                target_datetime='2024:01:15 14:30:00'
            )
            
            updated = db.get_task(task.id)
            assert updated.status == 'success'
            assert updated.target_path == '/test/output.heic'
            assert updated.metadata_preserved is True
    
    def test_get_stats(self):
        """Test statistics retrieval"""
        with tempfile.TemporaryDirectory() as tmpdir:
            db_path = os.path.join(tmpdir, "test.db")
            db = Database(db_path)
            
            # Create some tasks
            task1 = db.create_task(task_type='once', source_path='/test/1.jpg')
            db.update_task(task1.id, status='success', metadata_preserved=True)
            
            task2 = db.create_task(task_type='once', source_path='/test/2.jpg')
            db.update_task(task2.id, status='failed')
            
            task3 = db.create_task(task_type='watch', source_path='/test/3.jpg')
            db.update_task(task3.id, status='success', metadata_preserved=False)
            
            stats = db.get_stats()
            
            assert stats['total'] == 3
            assert stats['success'] == 2
            assert stats['failed'] == 1
            assert stats['metadata_preserved'] == 1
            assert stats['metadata_preservation_rate'] == 50.0


if __name__ == '__main__':
    pytest.main([__file__, '-v'])
