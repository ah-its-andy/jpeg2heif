# Workflow Converter Feature

## Overview

The Workflow Converter feature allows you to define custom media conversion workflows using YAML configuration files. These workflows can execute shell commands in sequence, with support for template variables, timeouts, and metadata preservation.

## YAML Workflow Specification

### Basic Structure

```yaml
name: workflow-name
description: "Description of what this workflow does"
runs-on: shell  # Currently supports: shell, docker (future)
timeout: 120    # Global timeout in seconds

can_convert:    # Optional: specify which files can be converted
  extensions: [".jpg", ".jpeg"]  # Method 1: List of file extensions
  # OR
  run: |                         # Method 2: Script that returns exit code 0 if supported
    file_type=$(file -b --mime-type "{{INPUT_FILE}}")
    [[ "$file_type" == "image/jpeg" ]] && exit 0 || exit 1
  timeout: 5                     # Timeout for run script (default: 10s)

env:
  VARIABLE_NAME: "{{TEMPLATE_VAR}}"

steps:
  - name: step-name
    run: |
      command to execute
      can be multiple lines
    workdir: "{{TMP_DIR}}"  # Optional working directory
    timeout: 60             # Step-level timeout in seconds
    env:                    # Step-level environment variables
      STEP_VAR: "value"

outputs:
  output_file: "{{TMP_OUTPUT}}"
```

### File Format Validation (can_convert)

The `can_convert` section is optional and allows you to specify which files can be converted by this workflow. You can use one of two methods:

#### Method 1: Extensions List

Specify a list of supported file extensions. The file extension is matched case-insensitively.

```yaml
can_convert:
  extensions: [".jpg", ".jpeg"]  # Supports both .jpg and .jpeg files
```

#### Method 2: Run Script

Execute a shell script that returns exit code 0 if the file is supported, or non-zero if not supported. This method allows for more complex validation logic.

```yaml
can_convert:
  run: |
    # Check MIME type using file command
    file_type=$(file -b --mime-type "{{INPUT_FILE}}")
    case "$file_type" in
      image/jpeg|image/png|image/gif)
        exit 0  # Supported
        ;;
      *)
        exit 1  # Not supported
        ;;
    esac
  timeout: 5  # Optional: timeout in seconds (default: 10)
```

**Important Notes:**
- You must specify either `extensions` OR `run`, not both
- The `run` script has access to all template variables (e.g., `{{INPUT_FILE}}`)
- If no `can_convert` is specified, the workflow is assumed to support all files
- Extensions should start with a dot (e.g., `.jpg` not `jpg`)

### Basic Structure

### Supported Template Variables

The following variables are automatically populated at runtime:

| Variable | Description | Example |
|----------|-------------|---------|
| `{{INPUT_FILE}}` | Full path to input file | `/path/to/photo.jpg` |
| `{{INPUT_DIR}}` | Directory containing input file | `/path/to` |
| `{{INPUT_BASENAME}}` | Filename without extension | `photo` |
| `{{INPUT_FILE_EXT}}` | File extension (lowercase) | `jpg` |
| `{{PARENT_DIR}}` | Parent directory of input dir | `/path` |
| `{{OUTPUT_FILE}}` | Final output file path | `/path/heic/photo.heic` |
| `{{TMP_DIR}}` | Temporary working directory | `/tmp/workflow-12345` |
| `{{TMP_OUTPUT}}` | Temporary output file path | `/tmp/workflow-12345/photo.heic` |
| `{{FILE_MD5}}` | MD5 hash of input file | `a1b2c3d4...` |
| `{{TIMESTAMP}}` | Unix timestamp | `1699999999` |
| `{{QUALITY}}` | Quality setting (1-100) | `85` |
| `{{CONVERT_QUALITY}}` | Alias for QUALITY | `85` |

### Example Workflows

#### JPEG to HEIC Conversion

```yaml
name: jpeg-to-heic
description: "Convert JPEG to HEIC with metadata preservation"
runs-on: shell
timeout: 120

can_convert:
  extensions: [".jpg", ".jpeg"]

env:
  QUALITY: "{{QUALITY}}"

steps:
  - name: convert-to-heic
    run: |
      magick "{{INPUT_FILE}}" -quality {{QUALITY}} "{{TMP_OUTPUT}}"
    workdir: "{{TMP_DIR}}"
    timeout: 60

  - name: copy-metadata
    run: |
      exiftool -TagsFromFile "{{INPUT_FILE}}" -overwrite_original "{{TMP_OUTPUT}}"
    timeout: 30

  - name: verify-metadata
    run: |
      echo "Verifying metadata..."
      exiftool -DateTimeOriginal -s -s -s "{{TMP_OUTPUT}}"
    timeout: 10

outputs:
  output_file: "{{TMP_OUTPUT}}"
```

#### PNG to AVIF Conversion

```yaml
name: png-to-avif
description: "Convert PNG to AVIF format"
runs-on: shell
timeout: 180

can_convert:
  extensions: [".png"]

env:
  QUALITY: "{{QUALITY}}"
  SPEED: "4"

steps:
  - name: convert-to-avif
    run: |
      avifenc --min 0 --max 63 --speed {{SPEED}} --jobs 4 \
        "{{INPUT_FILE}}" "{{TMP_OUTPUT}}"
    workdir: "{{TMP_DIR}}"
    timeout: 120

  - name: verify-output
    run: |
      file "{{TMP_OUTPUT}}"
      ls -lh "{{TMP_OUTPUT}}"
    timeout: 10

outputs:
  output_file: "{{TMP_OUTPUT}}"
```

#### Image to WebP with Script-based Validation

```yaml
name: image-to-webp
description: "Convert various image formats to WebP using script-based validation"
runs-on: shell
timeout: 120

can_convert:
  run: |
    # Check if file is a supported image format (JPEG, PNG, GIF, TIFF)
    file_type=$(file -b --mime-type "{{INPUT_FILE}}")
    case "$file_type" in
      image/jpeg|image/png|image/gif|image/tiff)
        exit 0  # Supported
        ;;
      *)
        exit 1  # Not supported
        ;;
    esac
  timeout: 5

env:
  QUALITY: "{{QUALITY}}"

steps:
  - name: convert-to-webp
    run: |
      cwebp -q {{QUALITY}} "{{INPUT_FILE}}" -o "{{TMP_OUTPUT}}"
    workdir: "{{TMP_DIR}}"
    timeout: 60

  - name: verify-output
    run: |
      echo "Verifying WebP output..."
      file "{{TMP_OUTPUT}}"
      ls -lh "{{TMP_OUTPUT}}"
    timeout: 10

outputs:
  output_file: "{{TMP_OUTPUT}}"
```

## Using the Web UI

### Creating a Workflow

1. Navigate to the **Workflows** tab in the dashboard
2. Click **Create Workflow**
3. Fill in the form:
   - **Name**: Unique identifier (e.g., `jpeg-to-heic`)
   - **Description**: Brief description
   - **YAML Configuration**: Paste or write your workflow YAML
   - **Enabled**: Check to enable immediately
4. Click **Validate** to check for errors
5. Click **Save** to create the workflow

### Managing Workflows

- **View**: Click the 👁️ icon to view workflow details
- **Edit**: Click the ✏️ icon to modify the workflow
- **Delete**: Click the 🗑️ icon to remove the workflow
- **Enable/Disable**: Edit the workflow and toggle the "Enabled" checkbox

### Viewing Workflow Runs

The **Recent Workflow Runs** section shows:
- Workflow name
- File path processed
- Status (running, success, failed)
- Duration
- Start time

Click **View** to see detailed logs including:
- Execution logs
- Standard output (stdout)
- Standard error (stderr)
- Metadata preservation status
- Step-by-step results

## API Endpoints

### Workflow Management

```bash
# List all workflows
GET /api/workflows?limit=100&offset=0

# Get specific workflow
GET /api/workflows/{id}

# Create workflow
POST /api/workflows
Content-Type: application/json
{
  "name": "my-workflow",
  "description": "Description",
  "yaml": "...",
  "enabled": true
}

# Update workflow
PUT /api/workflows/{id}
Content-Type: application/json
{
  "name": "my-workflow",
  "description": "Updated description",
  "yaml": "...",
  "enabled": true
}

# Delete workflow
DELETE /api/workflows/{id}

# Validate workflow YAML
POST /api/workflows/validate
Content-Type: application/json
{
  "yaml": "..."
}
```

### Workflow Runs

```bash
# List runs for a workflow
GET /api/workflows/{id}/runs?limit=50&offset=0

# Get specific run details
GET /api/workflows/runs/{run_id}

# Trigger workflow run (future)
POST /api/workflows/{id}/run
Content-Type: application/json
{
  "file_path": "/path/to/file.jpg",
  "variables": {
    "QUALITY": "90"
  },
  "dry_run": false
}
```

## Security Considerations

### Command Injection Prevention

- Template variables are shell-escaped automatically
- User input is validated before execution
- Commands run in isolated temporary directories

### Best Practices

1. **Run in Containers**: For production, run workflows in Docker containers with resource limits
2. **Use Non-Root Users**: Configure the application to run as a non-privileged user
3. **Enable Workflow Approval**: Only administrators should be able to create/edit workflows
4. **Audit Workflows**: Review workflow YAML before enabling
5. **Set Timeouts**: Always specify reasonable timeouts to prevent runaway processes
6. **Monitor Resources**: Track CPU, memory, and disk usage

### Dangerous Commands

Avoid or carefully review workflows containing:
- `rm -rf` or destructive file operations
- Network operations (`curl`, `wget`) to untrusted sources
- System modification commands (`apt-get`, `yum`)
- Commands that spawn background processes without cleanup

## Installation Requirements

### Required Command-Line Tools

Depending on your workflows, install these tools:

```bash
# For JPEG to HEIC
apt-get install -y imagemagick libheif-examples libimage-exiftool-perl

# For PNG to AVIF
apt-get install -y libavif-bin

# For general image processing
apt-get install -y imagemagick ffmpeg
```

### Docker Image

The provided Dockerfile includes common conversion tools. To add more:

```dockerfile
RUN apt-get update && apt-get install -y \
    your-conversion-tool \
    && rm -rf /var/lib/apt/lists/*
```

## Workflow Execution Flow

1. **Load Workflow**: Fetch from database and parse YAML
2. **Validate**: Check syntax and required fields
3. **Create Temp Dir**: Isolated workspace for execution
4. **Prepare Variables**: Calculate MD5, populate template vars
5. **Execute Steps**: Run each step sequentially
   - Replace template variables
   - Set working directory
   - Apply timeouts
   - Capture stdout/stderr
6. **Handle Outputs**: Copy generated files to final destination
7. **Extract Metadata**: Verify metadata preservation
8. **Update Database**: Record run results and logs
9. **Cleanup**: Remove temporary directory

## Troubleshooting

### Workflow Validation Errors

- **"workflow name is required"**: Add a `name` field
- **"at least one step is required"**: Add steps to the workflow
- **"runs-on must be 'shell' or 'docker'"**: Fix the `runs-on` value

### Runtime Errors

- **"command not found"**: Install required CLI tools
- **"permission denied"**: Check file permissions and user privileges
- **"timeout exceeded"**: Increase timeout values or optimize commands
- **"no such file or directory"**: Verify template variables and file paths

### Metadata Not Preserved

- Ensure `exiftool` is installed
- Check that source file has EXIF data
- Verify output format supports metadata
- Review workflow logs for exiftool errors

## Performance Tuning

- **Parallel Steps**: Not yet supported, but planned for future releases
- **Resource Limits**: Use Docker with CPU/memory limits
- **Disk I/O**: Use fast storage for temp directories
- **Worker Pool**: Adjust `MAX_WORKERS` in configuration

## Future Enhancements

- [ ] Docker container execution (`runs-on: docker`)
- [ ] Conditional steps (`if` conditions)
- [ ] Step dependencies and parallel execution
- [ ] Workflow templates and inheritance
- [ ] Variable validation and type checking
- [ ] Retry logic for failed steps
- [ ] Notification on completion/failure
- [ ] Resource usage monitoring
- [ ] Workflow versioning and rollback
- [ ] Import/export workflow bundles
