# DataSentinel 数据哨兵

轻量级 Windows 敏感数据扫描与分类分级工具。单文件运行，无需安装，自动识别文件中的个人信息、密钥凭证等敏感数据，提供可视化界面与报告导出。

## 功能特性

- **10 种内置检测规则** — 身份证号、手机号、银行卡号、邮箱、IP 地址、统一社会信用代码、AWS 密钥、GitHub Token、私钥文件、高熵密钥
- **5 级分类标准** — L1 公开 → L2 内部 → L3 机密 → L4 秘密 → L5 管控
- **19 种文件格式** — 纯文本 (.txt/.csv/.log/.json/.xml/.md/.env/.yaml/.yml/.ini/.conf/.toml/.bat/.ps1/.sh/.sql) + Office (.docx/.xlsx/.pptx)
- **多核并行扫描** — 自动利用多核 CPU，实时进度推送
- **误报修正** — 支持单条和批量修正分类结果，导出报告包含自动识别结果与修正结果
- **可视化统计** — 级别分布环形图、类别分布条形图、汇总卡片
- **报告导出** — JSON（程序处理） / CSV（Excel 查看，含 UTF-8 BOM）
- **纯本地运行** — 所有扫描和分析在本地完成，不联网、不上传数据
- **单文件交付** — 编译为 ~7MB 的 .exe，双击即可使用，自动打开浏览器界面

## 快速开始

### 下载使用

1. 从 [Releases](../../releases) 下载最新的 `datasentinel.exe`
2. 双击运行，浏览器自动打开操作界面
3. 如果浏览器未自动打开，查看程序同目录下 `datasentinel.log` 获取访问地址

### 使用步骤

1. **选择目标目录** — 输入路径或点击浏览按钮选择要扫描的文件夹
2. **配置扫描参数** — 选择文件类型和检测策略
3. **开始扫描** — 等待进度完成
4. **查看结果** — 按级别筛选、排序，点击"查看"查看匹配详情
5. **修正误报** — 单条或批量修正分类级别
6. **导出报告** — 下载 JSON 或 CSV 报告

## 编译

**环境要求**: Go 1.21+

```bash
# 标准编译（带控制台窗口，便于调试）
go build -o datasentinel.exe .

# 发布编译（无控制台窗口，适合分发）
go build -ldflags="-s -w -H windowsgui" -o datasentinel.exe .
```

## 技术架构

```
WalkFiles (goroutine) ──→ Worker Pool (N=NumCPU) ──→ Result Channel ──→ Report
  递归遍历目录               ExtractText()                收集+汇总         SSE推送
  按扩展名过滤               Rule.Match()                 computeSummary
                             Classify() → FileResult
```

- **Go 后端** — HTTP 服务 + goroutine 并发管道 + context 取消
- **系统浏览器 UI** — Go 启动本地 HTTP 服务，自动打开默认浏览器
- **SSE 实时推送** — Server-Sent Events 推送扫描进度
- **embed.FS** — HTML/CSS/JS 嵌入二进制，单文件交付
- **纯标准库** — Office 文档解析使用 `archive/zip` + `encoding/xml`，无重量级依赖

## 项目结构

```
├── main.go              # 入口：启动 HTTP 服务 + 打开浏览器
├── model/types.go       # 数据结构：Level, Match, FileResult, ScanReport
├── rules/
│   ├── rules.go         # 规则定义、正则匹配、脱敏处理
│   ├── patterns.go      # 10 条 MVP 检测规则
│   ├── validators.go    # 校验函数：身份证 Mod11-2、Luhn、USCC Mod31、Shannon 熵
│   └── classifier.go    # 匹配结果 → L1-L5 分级
├── scanner/
│   ├── walker.go        # 递归遍历 + 扩展名过滤
│   ├── extractor.go     # 纯文本/DOCX/XLSX/PPTX 内容提取
│   └── pipeline.go      # 并发扫描管道
├── server/
│   ├── server.go        # HTTP 路由 + API 处理 + 误报修正
│   └── handlers.go      # JSON/CSV 输出工具函数
└── ui/
    ├── embed.go         # 资源嵌入
    ├── index.html       # 单页应用
    ├── app.js           # 前端逻辑
    └── style.css        # 样式 + 暗色/亮色主题
```

## 检测规则说明

| 规则 | 校验方式 | 分类级别 |
|------|---------|---------|
| 身份证号 | GB 11643-1999 Mod11-2 校验 | L3 |
| 手机号 | 1[3-9] 开头 11 位格式 | L3 |
| 银行卡号 | Luhn/Mod-10 校验 (13-19 位) | L3 |
| 邮箱地址 | 标准邮箱格式 | L2 |
| IPv4 地址 | 各段 0-255 校验 | L2 |
| 统一社会信用代码 | GB 32100-2015 Mod31 校验 | L3 |
| AWS Access Key | AKIA 开头 20 位 | L5 |
| GitHub Token | ghp_ 开头 36 位 | L5 |
| 私钥文件 | PEM 格式私钥块 | L5 |
| 高熵密钥 | Shannon 熵 > 4.5 | L5 |

## 分级标准

| 级别 | 名称 | 含义 |
|------|------|------|
| L1 | 公开 (Public) | 未发现敏感数据 |
| L2 | 内部 (Internal) | 包含邮箱、IP 等一般性信息 |
| L3 | 机密 (Confidential) | 包含身份证号、手机号、银行卡号等个人敏感信息 |
| L4 | 秘密 (Secret) | 同一文件包含 3 项及以上 L3 级别数据 |
| L5 | 管控 (Restricted) | 包含私钥、云服务密钥、高熵密钥等核心凭证 |

## License

[MIT](LICENSE)
