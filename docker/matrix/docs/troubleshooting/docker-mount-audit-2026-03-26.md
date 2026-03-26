# 🔍 HotPlex Docker 配置排查总结

**排查时间**: 2026-03-26  
**排查范围**: 容器环境变量继承 + 配置文件挂载完整性

---

## 🎯 排查触发原因

用户报告两个问题：
1. 容器环境 hotplex 用户未能继承 envfile 的环境变量
2. 容器内 `~/.claude/plugins/known_marketplaces.json` 文件不完整

---

## 🔧 已修复问题 (共 5 个)

### 第一轮排查

#### 1. ✅ **环境变量优先级冲突**
**问题**: Docker Compose `environment` section 覆盖了 `env_file` 配置  
**影响**: `.env-01/02/03` 中的 `GIT_USER_NAME/EMAIL` 不生效  
**修复**: 从 `common.yml` 移除 `GIT_USER_*` 变量定义  
**验证**: 
```
hotplex-01: GIT_USER_NAME=HotPlex-1 ✅
hotplex-02: GIT_USER_NAME=HotPlex-2 ✅
hotplex-03: GIT_USER_NAME=HotPlex-3 ✅
```

#### 2. ✅ **plugins/ 目录挂载缺失**
**问题**: `common.yml` 未挂载 `plugins/` 目录  
**影响**: `known_marketplaces.json` 不完整  
**修复**: 添加 `plugins/` 目录挂载  
**验证**: 34 行完整文件，4 个 marketplace 可用

---

### 第二轮排查 (举一反三)

#### 3. ✅ **settings.local.json 权限配置缺失**
**问题**: 容器内缺少 128 个权限配置  
**影响**: Claude Code 功能受限，无法执行必要的 Bash 命令  
**修复**: 添加 `settings.local.json` 文件挂载  
**验证**: 
- MD5 校验和: `7c773e5b5f9dcf04987d5e77c5544c0d` ✅
- 权限数量: 128 项 allow ✅

#### 4. ✅ **hooks/ 目录挂载缺失**
**问题**: 容器内缺少 hooks 脚本  
**影响**: prompt 优化自动注入角色功能失效  
**修复**: 添加 `hooks/` 目录挂载  
**验证**: `inject-role-on-prompt-optimization.py` 可用

#### 5. ✅ **容器重建方式错误**
**问题**: `docker compose restart` 不会重新读取 env_file  
**影响**: 环境变量变更不生效  
**修复**: 使用 `docker compose down + up` 完全重建  
**验证**: 所有容器状态 healthy ✅

---

## 📊 最终配置状态

### Docker Compose 挂载配置

```yaml
volumes:
  # Claude config from preprocessed seed (read-only, container-compatible)
  - ${HOME}/.hotplex/claude-seed/settings.json:/home/hotplex/.claude/settings.json:ro
  - ${HOME}/.hotplex/claude-seed/settings.local.json:/home/hotplex/.claude/settings.local.json:ro  # ⭐ 新增
  - ${HOME}/.hotplex/claude-seed/skills:/home/hotplex/.claude/skills:ro
  - ${HOME}/.hotplex/claude-seed/scripts:/home/hotplex/.claude/scripts:ro
  - ${HOME}/.hotplex/claude-seed/plugins:/home/hotplex/.claude/plugins:ro              # ⭐ 新增
  - ${HOME}/.hotplex/claude-seed/hooks:/home/hotplex/.claude/hooks:ro                  # ⭐ 新增
  - ${HOME}/.hotplex/claude-seed/statusline.sh:/home/hotplex/.claude/statusline.sh:ro
```

### 挂载完整性验证

| 配置项 | 类型 | 文件数/大小 | 状态 |
|--------|------|-----------|------|
| settings.json | 文件 | 4.2K | ✅ |
| settings.local.json | 文件 | 5.3K (128 权限) | ✅ ⭐ |
| skills/ | 目录 | 15,066 文件 | ✅ |
| scripts/ | 目录 | 4 文件 | ✅ |
| plugins/ | 目录 | 5,087 文件 | ✅ ⭐ |
| hooks/ | 目录 | 1 文件 | ✅ ⭐ |
| statusline.sh | 文件 | 290B | ✅ |

### 容器运行状态

```
NAME         STATUS                   HEALTH
hotplex-01   Up 2 minutes (healthy)   ✅
hotplex-02   Up 2 minutes (healthy)   ✅
hotplex-03   Up 2 minutes (healthy)   ✅
```

---

## 🎓 排查方法论

### 1. 环境变量问题
```
问题假设 → 优先级检查 → 配置对比 → 根因定位 → 修复验证
```

**工具使用**:
- `docker exec <container> env` - 查看容器环境变量
- `comm -12 <(sort file1) <(sort file2)` - 文件差异对比
- `docker compose down + up` - 完全重建容器

### 2. 配置挂载问题
```
目录对比 → 文件检查 → MD5 校验 → 数量统计 → 功能验证
```

**工具使用**:
- `ls -la` - 目录结构对比
- `md5sum` - 文件完整性校验
- `find -type f | wc -l` - 文件数量统计
- `jq` - JSON 结构解析

---

## 📚 关键教训

### Docker Compose 环境变量优先级
```
1. environment section (硬编码值)        [最高]
2. environment section (${VAR} 替换)
3. env_file                               [最低]
4. Dockerfile ENV
```

**最佳实践**:
- ✅ `env_file` 用于实例特定配置 (token, bot ID, git identity)
- ✅ `environment` 用于通用配置 + 默认值
- ❌ 不要在两个地方定义同一个变量

### 容器配置更新流程
```bash
# ❌ 错误方式 - 不重新读取 env_file
docker compose restart

# ✅ 正确方式 - 完全重建
docker compose down && docker compose up -d

# ✅ 或一步到位
docker compose stop && docker compose rm -f && docker compose up -d
```

---

## 🚀 后续建议

### 自动化检查脚本
```bash
#!/bin/bash
# 保存为 scripts/verify-claude-mounts.sh

echo "=== 验证 Claude 配置挂载 ==="
for item in settings.json settings.local.json skills scripts plugins hooks statusline.sh; do
    if docker exec hotplex-01 test -e /home/hotplex/.claude/$item; then
        echo "✅ $item"
    else
        echo "❌ $item MISSING"
    fi
done
```

### CI/CD 集成
在 Makefile 中添加验证目标：
```makefile
.PHONY: verify-mounts
verify-mounts:
	@echo "Verifying Claude config mounts..."
	@for item in settings.json settings.local.json skills scripts plugins hooks statusline.sh; do \
		docker exec hotplex-01 test -e /home/hotplex/.claude/$item || (echo "❌ Missing: $$item" && exit 1); \
	done
	@echo "✅ All mounts verified"
```

---

## ✅ 最终结论

**所有问题已解决**：
- ✅ 环境变量正确继承（3 个实例各有独立配置）
- ✅ 配置文件完整挂载（7 个挂载点全部存在）
- ✅ 文件完整性验证通过（MD5 校验）
- ✅ 权限配置完整（128 项）
- ✅ 容器运行状态健康

**无遗漏配置项，系统运行正常。**
