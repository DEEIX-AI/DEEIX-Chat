# DeepSeek Search MCP

把 DeepSeek 的 [Search & Fetch API](https://api-docs.deepseek.com/guides/search) 包装成 MCP Streamable HTTP 服务，供 DEEIX Chat 的 MCP 工具系统调用。

## 工具

- `deepseek_search(query, count?, need_content?, ori_question?)`
- `deepseek_fetch(url)`

## 运行

```bash
docker build -t deeix-deepseek-search:local .
docker run --rm -p 8080:8080 \
  -e DEEPSEEK_API_KEY=sk-... \
  deeix-deepseek-search:local
```

可选环境变量：

- `DEEPSEEK_BASE_URL`：默认 `https://api.deepseek.com`
- `MCP_AUTH_TOKEN`：设置后要求客户端携带 `Authorization: Bearer <token>`
- `PORT`：默认 `8080`

健康检查：`GET /healthz`。

## 在 DEEIX Chat 后台接入

1. 启动服务，确保 DEEIX 后端可以访问 `http://<host>:8080/mcp`。
2. 管理后台 → 工具 → 新建 MCP 服务：
   - 名称：DeepSeek Search
   - 地址：`http://mcp-deepseek-search:8080/mcp`（同 compose 网络）或实际地址
   - 鉴权 Token：如果设置了 `MCP_AUTH_TOKEN`，填同一个值
3. 保存后在服务上点击“同步工具”，应看到 `deepseek_search` 和 `deepseek_fetch`。
4. 可选：在工具编辑里设置单次调用价格；DeepSeek 官方价格为 $0.75 / 1000 次成功请求，即每次 750,000 nanousd。
5. 在聊天输入区勾选工具即可使用。

## 说明

- 搜索与抓取只按 DeepSeek 的成功请求计费；失败的请求不产生费用。
- 服务无状态，不保存会话；`Mcp-Session-Id` 可忽略。
- `need_content=true` 会返回页面正文，Token 消耗会明显增加，按需开启。