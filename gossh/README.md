# GoSSH - SSH批量执行工具

一个用Go编写的轻量级SSH批量执行工具，类似于Ansible的ad-hoc命令功能。

## 功能特性

- 通过SSH批量连接多台服务器执行命令
- 支持密码认证和SSH密钥认证
- 支持密钥密码保护
- 通过YAML配置文件管理服务器
- 支持服务器分组和标签筛选
- **支持命令行直接连接单台服务器（无需配置文件）**
- **本机执行支持（localhost/127.0.0.1）无需认证**
- **文件批量传输（本地→远程、远程→本地）**
- **本地脚本在远程服务器上执行**
- **日志保存到文件**
- **静默模式（简洁输出）**
- **交互式密码输入（安全）**
- 并发执行，可配置并发数
- 详细的执行结果输出

## 安装

```bash
# 克隆仓库
git clone <repository>
cd gossh

# 编译
make build

# 或者直接运行
make run
```

## 使用模式

GoSSH 支持四种使用模式：

1. **配置文件模式** - 批量管理多台服务器，支持分组、标签等功能
2. **直接连接模式** - 快速连接单台服务器，无需配置文件
3. **文件传输模式** - 在本地和远程之间传输文件
4. **脚本执行模式** - 将本地脚本上传到远程服务器并执行

## 模式1：直接连接（无需配置文件）

适用于临时连接单台服务器执行命令，无需创建配置文件。

### 本地执行（无需认证）

当目标主机为 `localhost`、`127.0.0.1` 或 `::1` 时，直接在本地执行命令，无需提供用户名和密码：

```bash
./gossh -h localhost "whoami"
./gossh -h 127.0.0.1 "pwd && ls -la"
./gossh -h localhost -v "cat /etc/os-release"
```

### 远程连接 - 使用密码

```bash
./gossh -h 192.168.1.1 -u root -p "your-password" "uptime"
./gossh -h 192.168.1.1 -P 2222 -u admin -p "xxx" "whoami"
```

**注意**：如果不使用 `-p` 参数且 `~/.ssh/id_rsa` 存在，工具会自动尝试使用默认密钥。

### 远程连接 - 使用密钥

如果不指定密码（`-p`）也不指定密钥路径（`--key`），工具会自动尝试使用默认密钥 `~/.ssh/id_rsa`：

```bash
./gossh -h 192.168.1.1 -u root "uptime"                    # 自动使用 ~/.ssh/id_rsa
./gossh -h 192.168.1.1 -u root --key ~/.ssh/id_rsa "df -h"  # 指定密钥路径
./gossh -h 192.168.1.1 -u root --key ~/.ssh/id_rsa --key-pass "passphrase" "ls -la"
```

如果默认密钥不存在，会交互式提示输入密码。

如果密钥连接失败，工具会显示友好的错误提示，帮助用户排查问题：
- 密钥文件是否存在或路径是否正确
- 密钥文件权限是否正确（应为 600）
- 服务器上是否配置了对应的公钥
- 密钥是否需要密码保护

### 直接连接参数说明

| 长参数 | 短参数 | 说明 | 必填 |
|--------|--------|------|------|
| `--host` | `-h` | 目标主机IP或域名 | 是 |
| `--port` | `-P` | SSH端口（默认22） | 否 |
| `--user` | `-u` | 用户名（localhost/127.0.0.1无需） | 远程连接时必填 |
| `--password` | `-p` | 密码（不指定则交互式输入） | 否（不指定则交互式提示） |
| `--key` | 无 | 私钥文件路径 | 远程连接时必填（或密码） |
| `--key-pass` | 无 | 私钥密码保护（如果私钥有密码） | 否 |
| `--timeout` | 无 | 连接超时时间（秒，默认10） | 否 |
| `--verbose` | `-v` | 显示详细输出 | 否 |

### 交互式密码输入（安全）

如果不使用 `-p` 参数指定密码，工具会交互式提示输入密码，密码不会显示在屏幕上：

```bash
./gossh -h 192.168.1.1 -u root "uptime"
请输入密码: [输入密码，不显示]
```

这种方式更安全，因为密码不会记录在命令历史中。

### 日志保存

使用 `--log` 参数将执行结果保存到文件：

```bash
./gossh -h 192.168.1.1 -u root -p "xxx" "uptime" --log result.log
./gossh -g web "df -h" --log /var/log/gossh-$(date +%Y%m%d).log
```

### 静默模式

使用 `-q` 或 `--quiet` 参数只显示简短状态，不显示详细输出：

```bash
./gossh -h 192.168.1.1 -u root -p "xxx" "uptime" -q
# 输出：
# 总计: 1 | 成功: 1 | 失败: 0
# [OK] 192.168.1.1
```

静默模式配合日志使用，可以在后台执行任务：

```bash
./gossh -h 192.168.1.1 -u root "uptime" -q --log result.log
```

## 模式2：文件传输

支持在本地和远程服务器之间传输文件。

### 上传文件（本地→远程）

```bash
./gossh -h 192.168.1.1 -u root -p "xxx" --put ./local-file.txt:/tmp/remote-file.txt
./gossh -h 192.168.1.1 -u root -p "xxx" --put ./data:/opt/data
```

### 下载文件（远程→本地）

```bash
./gossh -h 192.168.1.1 -u root -p "xxx" --get /etc/passwd:./passwd
./gossh -h 192.168.1.1 -u root -p "xxx" --get /var/log/nginx:/tmp/nginx-logs
```

### 本地文件操作

当目标为 localhost 时，文件操作在本地进行：

```bash
./gossh -h localhost --put ./source.txt:/tmp/dest.txt
./gossh -h localhost --get /etc/hosts:./hosts-backup
```

## 模式3：本地脚本在远程执行

将本地脚本上传到远程服务器，执行后自动清理。

```bash
./gossh -h 192.168.1.1 -u root -p "xxx" --script ./deploy.sh
./gossh -h 192.168.1.1 -u root -p "xxx" --script ./install.sh arg1 arg2
```

脚本会被上传到远程服务器的 `/tmp/` 目录，添加执行权限后运行，执行完成后自动删除。

## 模式4：配置文件模式

### 配置文件

创建 `config.yaml` 文件：

```yaml
# 全局SSH配置
global:
  port: 22
  timeout: 10
  concurrency: 10
  # 全局认证配置（可选）- 所有服务器会继承这些值
  user: root
  password: "your-password"
  private_key: "~/.ssh/id_rsa"
  private_key_passphrase: ""

# 服务器组定义
groups:
  web:
    # 这些服务器会继承全局的 user 和 private_key
    - host: 192.168.1.10
    - host: 192.168.1.11
      # 覆盖全局的 user
      user: admin
      password: "admin-pass"
  
  db:
    - host: 192.168.1.20
      # 覆盖全局的 private_key
      password: "db-password"
    - host: 192.168.1.21
      password: "db-password"

# 独立服务器定义
servers:
  # 继承所有全局认证配置
  - host: 192.168.1.100
    tags:
      - production
  
  # 覆盖部分全局配置
  - host: 192.168.1.101
    user: ubuntu
    private_key: "~/.ssh/ubuntu_key"
    tags:
      - production
```

### 配置说明

#### 全局配置 (`global`)

| 字段 | 说明 | 必填 |
|------|------|------|
| `port` | 默认SSH端口 | 否（默认22） |
| `timeout` | 连接超时时间（秒） | 否（默认10） |
| `concurrency` | 并发数 | 否（默认10） |
| `user` | 默认用户名 | 否 |
| `password` | 默认密码 | 否 |
| `private_key` | 默认SSH私钥路径 | 否 |
| `private_key_passphrase` | 默认密钥密码 | 否 |

**注意：** 全局认证配置是可选的。当服务器配置中没有指定认证信息时，会自动继承全局配置的值。

#### 服务器配置

| 字段 | 说明 | 必填 |
|------|------|------|
| `host` | 服务器地址 | 是 |
| `port` | SSH端口（默认使用全局配置或22） | 否 |
| `user` | 用户名（默认使用全局配置） | 否（如果没有全局user则为必填） |
| `password` | 密码（默认使用全局配置） | 否（密码或密钥至少需要一个） |
| `private_key` | SSH私钥路径（默认使用全局配置） | 否（密码或密钥至少需要一个） |
| `private_key_passphrase` | 密钥密码（默认使用全局配置） | 否 |
| `tags` | 标签列表 | 否 |

### 配置文件模式用法

#### 列出所有服务器

```bash
./gossh -l
./gossh --list
```

#### 在所有服务器上执行命令

```bash
./gossh "uptime"
./gossh "df -h"
./gossh -v "cat /etc/os-release"
```

#### 在指定组执行

```bash
./gossh -g web "systemctl status nginx"
./gossh --group db "mysql -V"
```

#### 按标签筛选执行

```bash
./gossh -t production "hostname"
./gossh --tags "production,web" "whoami"
```

#### 指定配置中的主机执行

```bash
./gossh --hosts 192.168.1.10,192.168.1.11 "uname -a"
```

#### 排除特定主机

```bash
./gossh -g web "uptime" --exclude 192.168.1.10          # 排除单台主机
./gossh "df -h" -e 192.168.1.10,192.168.1.11            # 排除多台主机
./gossh -t production "hostname" --exclude 192.168.1.20 # 按标签筛选时排除
```

#### 使用自定义配置文件

```bash
./gossh -c /path/to/config.yaml "ls -la"
./gossh --config /path/to/config.yaml "whoami"
```

#### 配置文件模式选项

| 长参数 | 短参数 | 说明 |
|--------|--------|------|
| `--config` | `-c` | 配置文件路径（默认config.yaml） |
| `--group` | `-g` | 服务器组名 |
| `--tags` | `-t` | 标签筛选（逗号分隔） |
| `--hosts` | 无 | 指定配置中的主机（逗号分隔） |
| `--exclude` | `-e` | 排除指定主机（逗号分隔） |
| `--verbose` | `-v` | 显示详细输出 |
| `--list` | `-l` | 列出所有服务器 |
| `--log` | 无 | 将输出保存到日志文件 |
| `--quiet` | `-q` | 静默模式，只显示简短状态 |

## 安全提示

- 建议使用SSH密钥认证而非密码
- 将私钥文件权限设置为 `600`
- 避免在配置文件或命令行中明文存储密码（命令行参数可能会被记录到shell历史）
- 工具使用 `InsecureIgnoreHostKey()` 跳过主机密钥验证，生产环境建议改进此行为
- **注意**：在命令行中使用 `-p`/`--password` 参数时，密码可能会被记录到shell历史中，建议在安全环境中使用
- 本机执行模式无需认证，方便快捷地在本地测试命令

## 示例输出

### 本地执行

```bash
$ ./gossh -h localhost "whoami"

========== 执行结果汇总 ==========
总计: 1 | 成功: 1 | 失败: 0
================================

[✓ SUCCESS] localhost
  命令: whoami
  输出:
    root
```

### 文件传输

```bash
$ ./gossh -h 192.168.1.1 -u root -p xxx --put ./deploy.sh:/tmp/deploy.sh

========== 执行结果汇总 ==========
总计: 1 | 成功: 1 | 失败: 0
================================

[✓ SUCCESS] 192.168.1.1
  命令: upload ./deploy.sh -> /tmp/deploy.sh
  输出:
    文件上传成功
```

### 脚本执行

```bash
$ ./gossh -h 192.168.1.1 -u root -p xxx --script ./setup.sh

========== 执行结果汇总 ==========
总计: 1 | 成功: 1 | 失败: 0
================================

[✓ SUCCESS] 192.168.1.1
  命令: script ./setup.sh
  输出:
    Installing packages...
    Done!
```

### 远程直接连接

```bash
$ ./gossh -h 192.168.1.1 -u root -p xxx "uptime"

========== 执行结果汇总 ==========
总计: 1 | 成功: 1 | 失败: 0
================================

[✓ SUCCESS] 192.168.1.1
  命令: uptime
  输出:
    14:30:00 up 10 days, 2:15, 1 user, load average: 0.52, 0.58, 0.59
```

### 配置文件模式批量执行

```bash
$ ./gossh -g web "uptime"

========== 执行结果汇总 ==========
总计: 3 | 成功: 3 | 失败: 0
================================

[✓ SUCCESS] 192.168.1.10
  命令: uptime
  输出:
    14:30:00 up 10 days, 2:15, 1 user, load average: 0.52, 0.58, 0.59

[✓ SUCCESS] 192.168.1.11
  命令: uptime
  输出:
    14:30:01 up 5 days, 8:42, 2 users, load average: 0.12, 0.18, 0.25

[✗ FAILED] 192.168.1.12
  命令: uptime
  退出码: 255
  错误: ssh: handshake failed: connection refused
```

### 静默模式

```bash
$ ./gossh -h 192.168.1.1 -u root -p xxx "uptime" -q
总计: 1 | 成功: 1 | 失败: 0
[OK] 192.168.1.1
```

## 开发

```bash
# 运行测试
make test

# 清理
make clean

# 下载依赖
make deps
```

## License

MIT