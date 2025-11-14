# Workflow can_convert 功能说明

## 概述

`can_convert` 节点用于指定workflow能够处理哪些类型的文件。在转换前，系统会先检查文件是否符合条件，只有通过检查的文件才会执行转换流程。

## 两种验证方式

### 方式1: 扩展名列表 (extensions)

通过指定支持的文件扩展名列表，系统会自动匹配文件扩展名。

```yaml
can_convert:
  extensions: [".jpg", ".jpeg"]
```

**特点：**
- 简单直接，适合基于文件扩展名的验证
- 扩展名匹配不区分大小写
- 扩展名必须以点(`.`)开头
- 可以指定多个扩展名

**示例：**
```yaml
name: jpeg-to-heic
description: "Convert JPEG to HEIC"
runs-on: shell
timeout: 120

can_convert:
  extensions: [".jpg", ".jpeg"]  # 支持 .jpg 和 .jpeg 文件

steps:
  - name: convert
    run: magick "{{INPUT_FILE}}" "{{TMP_OUTPUT}}"

outputs:
  output_file: "{{TMP_OUTPUT}}"
```

### 方式2: 执行脚本 (run)

通过执行shell脚本来判断文件是否可以转换。脚本返回exitcode 0表示支持，非0表示不支持。

```yaml
can_convert:
  run: |
    file_type=$(file -b --mime-type "{{INPUT_FILE}}")
    [[ "$file_type" == "image/jpeg" ]] && exit 0 || exit 1
  timeout: 5  # 可选，默认10秒
```

**特点：**
- 灵活强大，可以实现复杂的验证逻辑
- 可以使用所有模板变量 (如 `{{INPUT_FILE}}`)
- 可以调用系统命令 (如 `file`, `exiftool` 等)
- 支持超时设置 (默认10秒)

**示例：**
```yaml
name: image-to-webp
description: "Convert multiple image formats to WebP"
runs-on: shell
timeout: 120

can_convert:
  run: |
    # 检查文件的MIME类型
    file_type=$(file -b --mime-type "{{INPUT_FILE}}")
    case "$file_type" in
      image/jpeg|image/png|image/gif|image/tiff)
        exit 0  # 支持这些格式
        ;;
      *)
        exit 1  # 不支持其他格式
        ;;
    esac
  timeout: 5

steps:
  - name: convert-to-webp
    run: cwebp -q {{QUALITY}} "{{INPUT_FILE}}" -o "{{TMP_OUTPUT}}"

outputs:
  output_file: "{{TMP_OUTPUT}}"
```

## 验证规则

1. **只能选择一种方式**：必须指定 `extensions` 或 `run`，不能同时使用两者
2. **extensions 规则**：
   - 扩展名必须以点(`.`)开头，如 `.jpg` 而非 `jpg`
   - 支持多个扩展名
   - 匹配不区分大小写
3. **run 规则**：
   - 脚本必须返回exitcode 0表示支持
   - 脚本返回非0表示不支持
   - 可以设置timeout (默认10秒)
   - 脚本可以使用所有模板变量

## 高级示例

### 基于文件大小的验证

```yaml
can_convert:
  run: |
    # 只处理小于10MB的JPEG文件
    file_type=$(file -b --mime-type "{{INPUT_FILE}}")
    if [[ "$file_type" != "image/jpeg" ]]; then
      exit 1
    fi
    
    file_size=$(stat -f%z "{{INPUT_FILE}}" 2>/dev/null || stat -c%s "{{INPUT_FILE}}")
    if [ "$file_size" -gt 10485760 ]; then
      echo "File too large: $file_size bytes" >&2
      exit 1
    fi
    
    exit 0
  timeout: 5
```

### 基于EXIF数据的验证

```yaml
can_convert:
  run: |
    # 检查是否有EXIF DateTimeOriginal字段
    exiftool -DateTimeOriginal -s -s -s "{{INPUT_FILE}}" > /dev/null 2>&1
    if [ $? -eq 0 ]; then
      exit 0  # 有EXIF数据，支持
    else
      exit 1  # 没有EXIF数据，不支持
    fi
  timeout: 10
```

### 多条件组合验证

```yaml
can_convert:
  run: |
    # 1. 检查MIME类型
    file_type=$(file -b --mime-type "{{INPUT_FILE}}")
    if [[ ! "$file_type" =~ ^image/(jpeg|png)$ ]]; then
      echo "Unsupported format: $file_type" >&2
      exit 1
    fi
    
    # 2. 检查文件大小
    file_size=$(stat -f%z "{{INPUT_FILE}}" 2>/dev/null || stat -c%s "{{INPUT_FILE}}")
    if [ "$file_size" -lt 1024 ]; then
      echo "File too small: $file_size bytes" >&2
      exit 1
    fi
    
    # 3. 检查图片尺寸
    width=$(identify -format "%w" "{{INPUT_FILE}}" 2>/dev/null)
    if [ "$width" -lt 100 ]; then
      echo "Image width too small: $width pixels" >&2
      exit 1
    fi
    
    exit 0
  timeout: 10
```

## 可用的模板变量

在 `can_convert.run` 脚本中可以使用以下变量：

| 变量 | 说明 | 示例 |
|------|------|------|
| `{{INPUT_FILE}}` | 输入文件完整路径 | `/path/to/photo.jpg` |
| `{{INPUT_DIR}}` | 输入文件所在目录 | `/path/to` |
| `{{INPUT_BASENAME}}` | 文件名（不含扩展名） | `photo` |
| `{{INPUT_FILE_EXT}}` | 文件扩展名（小写，不含点） | `jpg` |
| `{{FILE_MD5}}` | 文件MD5哈希值 | `a1b2c3d4...` |
| `{{TMP_DIR}}` | 临时工作目录 | `/tmp/workflow-12345` |

## 注意事项

1. **性能考虑**：`can_convert` 检查会在每个文件转换前执行，应尽量保持快速
2. **超时设置**：建议设置合理的超时时间，避免脚本hang住
3. **错误处理**：脚本执行失败（非exitcode导致）会被认为不支持
4. **日志输出**：脚本的stderr输出会被记录，可用于调试
5. **可选节点**：如果不指定 `can_convert`，workflow会尝试处理所有文件

## 测试

可以使用API端点验证workflow配置：

```bash
curl -X POST http://localhost:8080/api/workflows/validate \
  -H "Content-Type: application/json" \
  -d @workflow.yaml
```

响应会包含验证结果和错误信息（如果有）。
