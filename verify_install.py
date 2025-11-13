#!/usr/bin/env python3
"""
Verify JPEG to HEIC converter installation
Checks all dependencies and system requirements
"""
import sys
import subprocess
import importlib

def check_python_version():
    """Check Python version"""
    print("Checking Python version...")
    version = sys.version_info
    if version.major >= 3 and version.minor >= 10:
        print(f"  ✓ Python {version.major}.{version.minor}.{version.micro}")
        return True
    else:
        print(f"  ✗ Python {version.major}.{version.minor}.{version.micro} (need 3.10+)")
        return False

def check_python_package(package_name, import_name=None):
    """Check if Python package is installed"""
    import_name = import_name or package_name
    try:
        mod = importlib.import_module(import_name)
        version = getattr(mod, '__version__', 'unknown')
        print(f"  ✓ {package_name} ({version})")
        return True
    except ImportError:
        print(f"  ✗ {package_name} not installed")
        return False

def check_system_library(lib_name, check_cmd=None):
    """Check if system library is available"""
    try:
        if check_cmd:
            result = subprocess.run(check_cmd, 
                                  capture_output=True, 
                                  text=True,
                                  timeout=5)
            if result.returncode == 0:
                print(f"  ✓ {lib_name}")
                return True
        else:
            # Default check for library files
            result = subprocess.run(['ldconfig', '-p'], 
                                  capture_output=True, 
                                  text=True,
                                  timeout=5)
            if lib_name.lower() in result.stdout.lower():
                print(f"  ✓ {lib_name}")
                return True
        
        print(f"  ✗ {lib_name} not found")
        return False
    except Exception as e:
        print(f"  ? {lib_name} (cannot verify: {e})")
        return None

def main():
    print("=" * 60)
    print("JPEG to HEIC Converter - Installation Verification")
    print("=" * 60)
    print()
    
    all_ok = True
    
    # Python version
    all_ok &= check_python_version()
    print()
    
    # Python packages
    print("Checking Python packages...")
    packages = [
        ('Pillow', 'PIL'),
        ('pillow-heif', 'pillow_heif'),
        ('piexif', 'piexif'),
        ('FastAPI', 'fastapi'),
        ('uvicorn', 'uvicorn'),
        ('watchdog', 'watchdog'),
        ('SQLAlchemy', 'sqlalchemy'),
        ('python-dotenv', 'dotenv'),
    ]
    
    for pkg_name, import_name in packages:
        result = check_python_package(pkg_name, import_name)
        if result is False:
            all_ok = False
    print()
    
    # Test pillow-heif HEIF support
    print("Testing pillow-heif HEIF support...")
    try:
        import pillow_heif
        pillow_heif.register_heif_opener()
        from PIL import Image
        print("  ✓ pillow-heif HEIF support available")
    except Exception as e:
        print(f"  ✗ pillow-heif error: {e}")
        all_ok = False
    print()
    
    # System libraries (Docker will have these)
    print("Checking system libraries (if running locally)...")
    print("  (These checks may fail outside Docker - that's OK)")
    
    libs = [
        'libheif',
        'libde265',
        'libx265',
        'libexif'
    ]
    
    for lib in libs:
        check_system_library(lib)
    print()
    
    # Final result
    print("=" * 60)
    if all_ok:
        print("✓ All critical dependencies installed!")
        print()
        print("Ready to run:")
        print("  python -m app.main")
        print()
        print("Or with Docker:")
        print("  docker-compose up")
    else:
        print("✗ Some dependencies are missing")
        print()
        print("Install missing packages:")
        print("  pip install -r requirements.txt")
        print()
        print("For system libraries, use Docker:")
        print("  docker-compose build")
    print("=" * 60)
    
    return 0 if all_ok else 1

if __name__ == '__main__':
    sys.exit(main())
