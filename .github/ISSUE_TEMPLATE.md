## 背景

### 问题描述

IntentRouter (brain/router.go) 存在两个核心问题：

1. **Cache Key 碰撞风险** (router.go:89)
   - 当前使用消息前 100 字符作为缓存 key
   - 不同消息可能共享相同的缓存 key，导致错误的路由结果
   - 示例："帮我查一下..." 和 "帮我写个..." 前100字符相同时会产生误缓存

2. **规则硬编码**
   - Fast-path 规则写死在代码中 (router.go:45-60)
   - 新场景需要修改代码、重新部署
   - 无法适配业务变化

### 影响范围

- 路由命中率不准确
- 规则更新需要发版
- 维护成本高

---

## 方案

### 1. Cache Key 碰撞修复

**方案选择**: 使用 SHA256 哈希作为缓存 key

```go
import "crypto/sha256"

// 缓存 key 生成
func generateCacheKey(msg string) string {
    h := sha256.New()
    h.Write([]byte(msg))
    return fmt.Sprintf("%x", h.Sum(nil))[:16] // 取前16字符
}
```

**优点**:
- 碰撞概率极低 (SHA256)
- 固定长度，内存占用可控
- 实现简单

### 2. 规则引擎化

**方案**: 将规则外置到配置文件，支持运行时热更新

```yaml
# brain/rules.yaml
fast_path_rules:
  - name: help_intent
    patterns:
      - "帮我"
      - "帮我查"
      - "请问"
    intent: help
    confidence: 0.9

  - name: code_intent
    patterns:
      - "写个"
      - "写一段"
      - "代码"
    intent: code
    confidence: 0.85
```

**配置加载**:

```go
type FastPathRule struct {
    Name       string   `yaml:"name"`
    Patterns   []string `yaml:"patterns"`
    Intent     string   `yaml:"intent"`
    Confidence float64  `yaml:"confidence"`
}

func LoadRules(path string) ([]FastPathRule, error) {
    data, err := os.ReadFile(path)
    // ... yaml 解析
}
```

---

## 实现计划

- [ ] 添加 crypto/sha256 依赖 (标准库，无需额外依赖)
- [ ] 修改 router.go generateCacheKey() 使用 SHA256
- [ ] 创建 brain/rules.yaml 配置文件
- [ ] 实现 LoadRules() 配置加载
- [ ] 重构 MatchFastPath() 使用配置化规则
- [ ] 添加单元测试

---

## 验收标准

1. 相同消息内容 → 相同缓存 key
2. 不同消息内容 → 极大概率不同缓存 key
3. 规则修改无需重新编译，通过配置文件即可生效
4. 单元测试覆盖关键路径

---

## 相关问题

- Related to PR #228
- Part of: Brain 三大组件优化
