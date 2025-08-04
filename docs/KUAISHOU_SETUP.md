# Kuaishou 视频下载配置指南

## 问题说明

Kuaishou (快手) 使用了现代化的反爬虫保护机制，包括：
- 风控系统拦截未认证的 API 请求
- 单页应用架构，视频数据通过 JavaScript 动态加载
- 需要有效的浏览器会话 cookies

## 解决方案

### 方法 1: 配置浏览器 Cookies (推荐)

1. **获取 Cookies**：
   - 在浏览器中访问 https://www.kuaishou.com
   - 登录您的快手账号(可选，但能提高成功率)
   - 打开开发者工具 (F12)
   - 转到 Application 标签页 -> Cookies
   - 复制所有 kuaishou.com 相关的 cookies

2. **配置 cookies**：
   编辑 `config/config.yaml` 文件：
   ```yaml
   platforms:
     kuaishou:
       enabled: true
       # 将浏览器 cookies 粘贴到这里，格式如：did=web_xxx; kpf=PC_WEB; kpn=KUAISHOU_VISION
       cookie: "did=web_35887ac65357c11bd42b25ed140ea495; kpf=PC_WEB; kpn=KUAISHOU_VISION; clientid=3"
       user_agent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
   ```

3. **重新测试**：
   ```bash
   ./bin/video-downloader download https://www.kuaishou.com/short-video/3xevd5h94x9qzqi
   ```

### 方法 2: 浏览器扩展 (替代方案)

如果配置 cookies 后仍然失败，建议使用专门的浏览器扩展：
- Video DownloadHelper
- SaveFrom.net helper
- 其他支持快手的下载扩展

## 当前实现状态

✅ **已实现**：
- GraphQL API 调用框架
- 完整的错误处理和回退机制
- 详细的日志记录便于调试

❌ **当前限制**：
- 需要有效的浏览器 cookies
- API 请求会被风控系统拦截（无 cookies 时）

## 技术细节

### 实现的功能
1. **双重提取策略**：首先尝试 GraphQL API，失败时回退到 HTML 解析
2. **风控检测**：能够识别和记录风控拦截情况
3. **Cookie 管理**：支持配置和使用浏览器 cookies
4. **详细日志**：提供完整的调试信息

### API 调用示例
当配置了有效 cookies 后，程序会：
1. 调用 `https://www.kuaishou.com/graphql`
2. 使用 `visionVideoDetail` 查询获取视频信息
3. 提取 `mainMvUrls` 中的真实视频下载链接
4. 下载真实的 MP4 文件而不是说明文件

## 故障排除

### 如果仍然下载失败：
1. 检查 cookies 是否有效（可能过期）
2. 确保 User-Agent 与浏览器匹配
3. 查看日志中的详细错误信息
4. 尝试使用不同的视频 URL

### 日志分析：
- `Making GraphQL API request` - API 调用开始
- `has_cookie:true` - 确认 cookies 已配置
- `GraphQL API returned non-success status` - API 被拦截
- `Successfully extracted video using GraphQL API` - 成功提取

## 示例配置

完整的 `config/config.yaml` 配置示例：
```yaml
platforms:
  kuaishou:
    enabled: true
    cookie: "did=web_xxxxx; kpf=PC_WEB; kpn=KUAISHOU_VISION; clientid=3; kuaishou.server.web_st=xxxxx"
    user_agent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
```

## 注意事项

- Cookies 可能会过期，需要定期更新
- 快手可能会更新反爬虫策略
- 建议遵守平台的使用条款和服务协议