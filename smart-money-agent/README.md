# Smart Money Agent

Go 实现的多链聪明钱地址资金流监控工具。无第三方 Go 依赖，支持单次采集、每天自动执行、漏跑补采、本地 JSON 存储及 Markdown 日报。需要 Go 1.22+；采集进程支持 Linux、macOS 和 BSD。

## 监控范围

默认配置提供 Ethereum、Base、Arbitrum、Optimism、Polygon、BNB Chain，使用 Etherscan V2。每条链单独配置钱包白名单；工具不会自动识别或认证“聪明钱”。请根据研究结果维护地址和标签，删除不用的链。

- ERC-20：按合约区分资产，统计 Transfer 记录中的流入、流出和净流入。
- 原生币：仅成功普通交易的 `value`，不包含内部调用、Gas、验证者奖励等，因此不能当作完整余额变动。
- 每个钱包独立统计；监控钱包互转会分别出现在双方日报中，自身转账保留记录但不计资金流。不能把钱包总流入当作整个地址集合的外部流入。
- 按币种原始精度精确计算，不使用浮点数；不提供美元估值、交易盈亏或买卖意图判断。转入不一定代表买入，转出也不一定代表卖出。
- 暂不支持 Solana、BTC、NFT、rebase 余额变化和跨链交易关联。代币符号来自 API，可能伪造，应以链 ID 和合约地址识别资产。

Etherscan 的链支持、历史接口速率和付费权限可能随套餐变化。部分链可能需要付费套餐；可访问官网不代表 API Key 有访问权限。权限错误不会被当作“当天无交易”。

## 快速开始

在本目录运行：

```sh
go build -o smart-money-agent ./cmd/smart-money-agent
./smart-money-agent init
```

编辑生成的 `config.json`，为每条保留的链填写钱包，例如：

```json
{
  "name": "ethereum",
  "chain_id": "1",
  "native_symbol": "ETH",
  "wallets": [
    {"label": "我研究过的钱包", "address": "0x填入真实的40位十六进制地址"}
  ]
}
```

空钱包列表会报错，避免在没有实际监控目标时生成成功日报。配置中不要放密钥；通过环境变量提供 Etherscan V2 API Key：

```sh
# 从安全的环境配置注入实际密钥，不要提交到仓库
export ETHERSCAN_API_KEY='替换为你的密钥'
./smart-money-agent check  # 本地配置及密钥环境变量检查，不调用 API
./smart-money-agent run
./smart-money-agent report

# 指定已结束的自然日，重跑会原子替换该日日报
./smart-money-agent run -date 2026-11-08
./smart-money-agent report -date 2026-11-08 > report-2026-11-08.md
```

默认日期是配置时区的昨天。配置文件支持 `-config /absolute/path/config.json`，数据目录的相对路径始终相对于配置文件所在目录，不随启动工作目录变化。

## 每天自动运行

```sh
./smart-money-agent daemon -config /absolute/path/config.json
```

默认在 `Asia/Shanghai` 每天 `08:00` 后采集前一天 `00:00` 到次日 `00:00`（不含结束时刻）的数据。守护进程每分钟检查一次，因此可能在计划时间后约一分钟启动。收到 SIGINT/SIGTERM 时取消网络请求并退出。

`catchup_days` 默认为 7：启动及运行时按从旧到新顺序检查最近 7 个已到执行时间的日期，跳过已完成日报；缺失、损坏或采集范围变更的日报会重采。首次启动会采集多天，产生相应 API 消耗。单日失败保留旧数据，并继续处理后续日期；本轮结束后等待 5 分钟再重试失败日期，成功日期不会重复采集。机器休眠时不会执行，唤醒后在补采窗口内补跑；更久的缺口使用 `run -date` 手动补采。

持续运行时请使用系统服务管理器托管进程并配置环境变量。也可只用系统 cron 调用 `run`，不要与 `daemon` 同时管理同一数据目录。例如 Linux 上 cron 系统时区为 Asia/Shanghai 时：

```cron
0 8 * * * /bin/sh -c '. /absolute/path/agent.env; exec /absolute/path/smart-money-agent run -config /absolute/path/config.json' >> /absolute/path/agent.log 2>&1
```

`agent.env` 中写 `export ETHERSCAN_API_KEY=...` 并设为仅本人可读；cron 不会自动继承交互终端环境。日志请配置轮转。cron 方式仅执行一次，不提供 daemon 的自动重试和补采。

## 配置与存储

完整模板见 `config.example.json`，也可以用 `init` 生成。`init` 不覆盖已有文件。

| 配置 | 默认 | 说明 |
| --- | --- | --- |
| `timezone` | Asia/Shanghai | 自然日和调度时区，内置时区数据库 |
| `daily_at` | 08:00 | 每天执行时间 HH:MM，建议留足链确认及索引延迟 |
| `data_dir` | data | 本地数据目录 |
| `api_key_env` | ETHERSCAN_API_KEY | 密钥环境变量名 |
| `request_interval_ms` | 550 | 串行请求最小间隔，最低 200ms，需符合账户套餐 |
| `page_size` | 1000 | 每页最多记录数，1–1000 |
| `max_pages` | 10 | 每个区块子范围的页数上限，1–1000；满页后自动拆分 |
| `catchup_days` | 7 | 守护进程补采窗口，1–365 |

`data/YYYY-MM-DD.json` 包含版本、采集时间、时区、配置指纹、地址名单、原始转账记录及逐钱包逐资产汇总。`raw_value` 为十进制整数串，汇总金额也是字符串。一次采集的所有链和钱包都成功后，才通过同目录临时文件、同步及 rename 替换日报；任何链失败都不保存半份日报。文件默认权限 0600，新建目录 0700。请自行备份，本工具不会自动清理历史记录。

API 网络错误、429、5xx 和速率限制最多尝试 4 次。分页达到 `max_pages`、10000 条查询窗口，或 API 返回查询超时/窗口过大时，会自动按区块二分重查。父范围的部分结果会丢弃，两个子范围不重叠，避免重复计数。若单一区块仍超限则明确失败，不保存可能缺失的日报；此时需要提高合理的分页容量或接入其他索引服务。重复分页、鉴权错误等不会通过拆分无限重试。

单进程顺序采集，整日日报在内存中组装，适合小规模地址名单。`run` 和 `daemon` 使用数据目录下的 `.collector.lock` 系统文件锁，阻止第二个采集进程并发运行；进程退出或崩溃后系统自动释放锁。锁文件会保留，请勿在运行中删除。`report` 可同时读取原子保存的完整日报。不依赖数据库；数据目录应放在支持文件锁的本地文件系统上。链重组、索引延迟或服务端漏数仍可能影响结果，已完成日报不会自动刷新，必要时手动 `run -date` 重采。切换监控地址名单会使补采窗口内日报重新生成，若需保留旧名单的结果，请使用不同 `data_dir`。

## 开发验证

```sh
go test -race ./...
go vet ./...
```

测试使用内存 HTTP transport，无需 API Key 或外网，覆盖金额精度、自转账、失败交易、日期边界、分页、重复页面、配置变更、原子保存、夏令时、区块自动拆分、子范围失败、跨进程锁、失败日期独立重试及 CLI 本地检查。真实 API 联调需自行填写名单和密钥。
