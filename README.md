<div align="center">

# DataSentinel

### 数据哨兵 — Sensitive Data Scanning & Classification Tool

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Platform: Windows](https://img.shields.io/badge/Platform-Windows-blue)]()
[![Go 1.21+](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)]()

A lightweight Windows tool for scanning and classifying sensitive data in files. Single executable, zero installation. Automatically detects personal information, credentials, and secrets, with a built-in web UI and report export.

轻量级 Windows 敏感数据扫描与分类分级工具。单文件运行，无需安装，自动识别文件中的个人信息、密钥凭证等敏感数据，提供可视化界面与报告导出。

</div>

---

## ✨ Features / 功能特性

| | |
|---|---|
| **10 Built-in Detection Rules** / 10 种内置检测规则 | ID cards, phone numbers, bank cards, emails, IPs, USCC, AWS keys, GitHub tokens, private keys, high-entropy secrets |
| **5-Level Classification** / 5 级分类标准 | L1 Public → L2 Internal → L3 Confidential → L4 Secret → L5 Restricted |
| **19 File Formats** / 19 种文件格式 | Plain text (.txt/.csv/.log/.json/.xml/.md/.env/.yaml/.yml/.ini/.conf/.toml/.bat/.ps1/.sh/.sql) + Office (.docx/.xlsx/.pptx) |
| **Multi-core Parallel Scanning** / 多核并行扫描 | Auto-utilizes multi-core CPU with real-time progress updates |
| **False Positive Correction** / 误报修正 | Single or batch correction of classification results; exports include both original and corrected results |
| **Visual Statistics** / 可视化统计 | Donut chart for level distribution, bar chart for category breakdown, summary cards |
| **Report Export** / 报告导出 | JSON (for programmatic use) / CSV (with UTF-8 BOM for Excel) |
| **Fully Local** / 纯本地运行 | All scanning and analysis runs locally — no internet, no data upload |
| **Single Executable** / 单文件交付 | Compiles to ~7 MB .exe, double-click to run, auto-opens browser UI |

## 🚀 Quick Start / 快速开始

### Download / 下载使用

1. Download the latest `datasentinel.exe` from [Releases](../../releases)
2. Double-click to run — the browser opens automatically

从 [Releases](../../releases) 下载最新的 `datasentinel.exe`，双击运行，浏览器自动打开操作界面。

> If the browser doesn't open automatically, check `datasentinel.log` in the same directory for the access URL.
>
> 如果浏览器未自动打开，查看程序同目录下 `datasentinel.log` 获取访问地址。

### Usage / 使用步骤

1. **Select target directory** / 选择目标目录 — Enter a path or browse for the folder to scan
2. **Configure scan parameters** / 配置扫描参数 — Choose file types and detection rules
3. **Start scan** / 开始扫描 — Wait for progress to complete
4. **View results** / 查看结果 — Filter by level, sort, click "View" for match details
5. **Correct false positives** / 修正误报 — Single or batch correction of classification levels
6. **Export report** / 导出报告 — Download JSON or CSV report

## 🔨 Build / 编译

**Requirement / 环境要求**: Go 1.21+

```bash
# Standard build (with console window for debugging)
# 标准编译（带控制台窗口，便于调试）
go build -o datasentinel.exe .

# Release build (no console window, for distribution)
# 发布编译（无控制台窗口，适合分发）
go build -ldflags="-s -w -H windowsgui" -o datasentinel.exe .
```

## 🏗️ Architecture / 技术架构

```
WalkFiles (goroutine) ──→ Worker Pool (N=NumCPU) ──→ Result Channel ──→ Report
  Recursive walk           ExtractText()              Collect+Summarize    SSE Push
  Extension filter          Rule.Match()               computeSummary
                            Classify() → FileResult
```

- **Go Backend** — HTTP server + goroutine concurrent pipeline + context cancellation
- **System Browser UI** — Local HTTP server with automatic browser launch
- **SSE Real-time Push** — Server-Sent Events for scan progress
- **embed.FS** — HTML/CSS/JS embedded in binary for single-file delivery
- **Standard Library Only** — Office document parsing via `archive/zip` + `encoding/xml`, no heavy dependencies

## 📁 Project Structure / 项目结构

```
├── main.go              # Entry point: HTTP server + browser launch / 入口：启动 HTTP 服务 + 打开浏览器
├── model/types.go       # Data structures: Level, Match, FileResult, ScanReport / 数据结构
├── rules/
│   ├── rules.go         # Rule definitions, regex matching, masking / 规则定义、正则匹配、脱敏处理
│   ├── patterns.go      # 10 MVP detection rules / 10 条检测规则
│   ├── validators.go    # Validators: ID Mod11-2, Luhn, USCC Mod31, Shannon entropy / 校验函数
│   └── classifier.go    # Match → L1-L5 classification / 匹配结果分级
├── scanner/
│   ├── walker.go        # Recursive walk + extension filter / 递归遍历 + 扩展名过滤
│   ├── extractor.go     # Plain text/DOCX/XLSX/PPTX content extraction / 内容提取
│   └── pipeline.go      # Concurrent scan pipeline / 并发扫描管道
├── server/
│   ├── server.go        # HTTP routes + API handlers + false positive correction / HTTP 路由 + API
│   └── handlers.go      # JSON/CSV output utilities / JSON/CSV 输出工具函数
└── ui/
    ├── embed.go         # Resource embedding / 资源嵌入
    ├── index.html       # Single-page application / 单页应用
    ├── app.js           # Frontend logic / 前端逻辑
    └── style.css        # Styling + dark/light theme / 样式 + 暗色/亮色主题
```

## 🔍 Detection Rules / 检测规则

| Rule / 规则 | Validation / 校验方式 | Level / 级别 |
|---|---|---|
| Chinese ID Card / 身份证号 | GB 11643-1999 Mod11-2 checksum | L3 |
| Phone Number / 手机号 | 1[3-9] prefix, 11-digit format | L3 |
| Bank Card / 银行卡号 | Luhn/Mod-10 algorithm (13-19 digits) | L3 |
| Email Address / 邮箱地址 | Standard email format | L2 |
| IPv4 Address / IPv4 地址 | Each octet 0-255 validation | L2 |
| USCC / 统一社会信用代码 | GB 32100-2015 Mod31 checksum | L3 |
| AWS Access Key | AKIA prefix, 20 characters | L5 |
| GitHub Token | ghp_ prefix, 36 characters | L5 |
| Private Key / 私钥文件 | PEM format private key block | L5 |
| High-entropy Secret / 高熵密钥 | Shannon entropy > 4.5 | L5 |

## 📊 Classification Levels / 分级标准

| Level / 级别 | Name / 名称 | Description / 含义 |
|---|---|---|
| L1 | Public / 公开 | No sensitive data found / 未发现敏感数据 |
| L2 | Internal / 内部 | Contains emails, IPs, etc. / 包含邮箱、IP 等一般性信息 |
| L3 | Confidential / 机密 | Contains IDs, phone numbers, bank cards / 包含身份证号、手机号、银行卡号等个人敏感信息 |
| L4 | Secret / 秘密 | 3+ L3 matches in the same file / 同一文件包含 3 项及以上 L3 级别数据 |
| L5 | Restricted / 管控 | Contains private keys, cloud API keys, high-entropy secrets / 包含私钥、云服务密钥、高熵密钥等核心凭证 |

## ❓ FAQ / 常见问题

<details>
<summary><strong>Is my data safe? / 我的数据安全吗？</strong></summary>

Yes. All scanning and analysis runs entirely on your local machine. No data is uploaded to any server.
完全安全。所有扫描和分析都在本地完成，不联网、不上传任何数据。
</details>

<details>
<summary><strong>Which OS is supported? / 支持哪些操作系统？</strong></summary>

Currently Windows only. The tool uses Windows-specific APIs for file system operations and browser launch.
目前仅支持 Windows，使用了 Windows 特有的文件系统操作和浏览器启动接口。
</details>

<details>
<summary><strong>What is the maximum file size? / 文件大小限制？</strong></summary>

50 MB per file. Larger files are automatically skipped.
单个文件最大 50 MB，超过此大小的文件会被自动跳过。
</details>

## 📄 License

[MIT](LICENSE)
