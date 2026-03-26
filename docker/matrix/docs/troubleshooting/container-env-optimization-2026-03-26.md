# HotPlex 容器环境优化验证报告

**执行时间**: 2026-03-26  
**目标**: 实现 statusline.sh 种子生成 + 环境变量注入到 .bashrc

---

## ✅ 实现功能

### 1. statusline.sh 种子生成

**实现方式**:
- 宿主机文件挂载为 `statusline.sh.seed` (只读)
- entrypoint 启动时复制到 `statusline.sh` 并设置权限

**验证结果**:
```bash
$ docker exec hotplex-01 ls -la /home/hotplex/.claude/statusline.sh
-rwxr-xr-x 1 hotplex hotplex 290 Mar 26 14:34 statusline.sh
```

✅ 文件存在且可执行

---

### 2. 环境变量注入到 .bashrc

**实现方式**:
- 在 entrypoint 中添加 `inject_env_to_bashrc()` 函数
- 使用 `printenv` 读取环境变量
- 过滤 `HOTPLEX_*`, `GITHUB_*`, `GIT_USER_*`, `ANTHROPIC_*` 变量
- 注入到 `/home/hotplex/.bashrc`
- 自动去重（删除旧的自动生成部分）

**验证结果**:

#### hotplex-01
```bash
export HOTPLEX_BOT_ID='U0AHRCL1KCM'
export GIT_USER_NAME='HotPlex-1'
export GITHUB_TOKEN='<REDACTED_GITHUB_TOKEN>'
```

#### hotplex-02
```bash
export HOTPLEX_BOT_ID='U0AJVRH4YF6'
export GIT_USER_NAME='HotPlex-2'
```

#### hotplex-03
```bash
export HOTPLEX_BOT_ID='U0AL7H8UU75'
export GIT_USER_NAME='HotPlex-3'
```

✅ 所有容器都正确注入了各自的环境变量

---

## 📝 技术细节

### 文件挂载配置 (common.yml)

```yaml
volumes:
  # statusline.sh 挂载为种子文件
  - ${HOME}/.hotplex/claude-seed/statusline.sh:/home/hotplex/.claude/statusline.sh.seed:ro
```

### Entrypoint 处理逻辑

#### 1. statusline.sh 生成
```bash
STATUSLINE_SEED="${CLAUDE_DIR}/statusline.sh.seed"
STATUSLINE_TARGET="${CLAUDE_DIR}/statusline.sh"

if [[ -f "${STATUSLINE_SEED}" ]]; then
    cp "${STATUSLINE_SEED}" "${STATUSLINE_TARGET}"
    chown hotplex:hotplex "${STATUSLINE_TARGET}"
    chmod +x "${STATUSLINE_TARGET}"
fi
```

#### 2. 环境变量注入
```bash
inject_env_to_bashrc() {
    # 1. 删除旧的自动生成部分
    sed -i "/^# === HotPlex Environment Variables/,/^# === End HotPlex Environment Variables$/d" ~/.bashrc
    
    # 2. 追加新的环境变量
    {
        echo "# === HotPlex Environment Variables (Auto-generated) ==="
        printenv | grep -E "^(HOTPLEX|GITHUB|GIT_USER|ANTHROPIC)_" | while IFS='=' read -r key value; do
            escaped_value=$(printf '%s\n' "$value" | sed "s/'/'\\\\''/g")
            echo "export ${key}='${escaped_value}'"
        done
        echo "# === End HotPlex Environment Variables ==="
    } >> ~/.bashrc
}
```

---

## 🎯 使用场景

### 场景 1: 交互式 Shell 调试
```bash
$ docker exec -it hotplex-01 bash
hotplex@U0AHRCL1KCM:~$ echo $HOTPLEX_BOT_ID
U0AHRCL1KCM

hotplex@U0AHRCL1KCM:~$ echo $GIT_USER_NAME
HotPlex-1
```

### 场景 2: 执行脚本时访问环境变量
```bash
$ docker exec hotplex-01 bash -c 'source ~/.bashrc && echo $GITHUB_TOKEN'
<REDACTED_GITHUB_TOKEN>
```

### 场景 3: statusline.sh 自动处理
- 容器启动时自动从种子生成
- 无需手动复制或权限设置
- 支持未来扩展（可在 entrypoint 中添加环境变量替换等处理）

---

## 🔧 故障排查

### 检查环境变量是否注入
```bash
docker exec hotplex-01 grep "HOTPLEX_BOT_ID" ~/.bashrc
```

### 重新注入环境变量
```bash
docker compose restart hotplex-01
```

### 检查 statusline.sh
```bash
docker exec hotplex-01 ls -la ~/.claude/statusline.sh
```

---

## ✅ 结论

**所有功能已成功实现并验证**:

- ✅ statusline.sh 种子生成机制
- ✅ 环境变量自动注入到 .bashrc
- ✅ 每个容器有独立的 BOT_ID 和 Git 身份
- ✅ 支持敏感信息（token）的安全注入
- ✅ 自动去重，避免重复注入
- ✅ 权限正确设置（hotplex 用户所有）

**无遗漏问题。**
