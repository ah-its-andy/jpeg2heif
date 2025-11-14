# 环境变量配置说明

## BUILTIN_CONVERTERS

控制要注册哪些内置转换器。

### 格式
逗号分隔的转换器名称列表。

### 可用的内置转换器
- `jpeg2heic` - JPEG 转 HEIC 转换器

### 示例

#### 注册 jpeg2heic 转换器
```bash
export BUILTIN_CONVERTERS="jpeg2heic"
```

#### 不注册任何内置转换器（仅使用workflow转换器）
```bash
export BUILTIN_CONVERTERS=""
# 或者不设置该环境变量
```

#### 注册多个转换器（未来扩展）
```bash
export BUILTIN_CONVERTERS="jpeg2heic,png2avif,webp2heic"
```

### 默认行为
如果不设置 `BUILTIN_CONVERTERS` 环境变量或设置为空字符串，则不会注册任何内置转换器。

### 使用场景

1. **仅使用Workflow转换器**
   ```bash
   # 不设置 BUILTIN_CONVERTERS
   ./jpeg2heif
   ```
   这种情况下，只有在数据库中配置的workflow转换器会被使用。

2. **同时使用内置和Workflow转换器**
   ```bash
   export BUILTIN_CONVERTERS="jpeg2heic"
   ./jpeg2heif
   ```
   这种情况下，内置的jpeg2heic转换器和数据库中的workflow转换器都会被使用。

3. **Docker环境**
   ```dockerfile
   ENV BUILTIN_CONVERTERS="jpeg2heic"
   ```
   
   或在docker-compose.yml中：
   ```yaml
   services:
     jpeg2heif:
       environment:
         - BUILTIN_CONVERTERS=jpeg2heic
   ```

### 优先级

在 `FindConverter` 查找转换器时的优先级顺序：
1. 首先检查已启用的内置转换器（从registry）
2. 然后检查已启用的workflow转换器（从数据库实时查询）

### 注意事项

1. **性能考虑**：Workflow转换器在每次查找时都会从数据库实时查询，而内置转换器在启动时注册一次。
2. **动态更新**：Workflow转换器支持运行时更新（通过Web界面），无需重启服务。内置转换器需要重启服务才能更改。
3. **环境变量格式**：转换器名称不区分大小写，但建议使用小写。空格会被自动去除。
4. **错误处理**：如果指定的转换器名称不存在，会在日志中显示警告，但不会中断启动。

### 查看已注册的转换器

通过Web界面或API查看：
```bash
curl http://localhost:8080/api/converters
```

响应示例：
```json
[
  {
    "name": "jpeg2heic",
    "type": "builtin",
    "target_format": "heic",
    "enabled": true
  },
  {
    "name": "workflow:jpeg-to-heic",
    "type": "workflow",
    "target_format": "heic",
    "enabled": true,
    "description": "Convert JPEG to HEIC with metadata preservation"
  }
]
```

### 启用/禁用转换器

可以通过Web界面或API动态启用/禁用转换器（包括内置和workflow转换器）：

```bash
# 禁用转换器
curl -X PUT http://localhost:8080/api/converters/jpeg2heic \
  -H "Content-Type: application/json" \
  -d '{"enabled": false}'

# 启用转换器
curl -X PUT http://localhost:8080/api/converters/workflow:jpeg-to-heic \
  -H "Content-Type: application/json" \
  -d '{"enabled": true}'
```
