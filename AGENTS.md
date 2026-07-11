# gotmux 开发须知（所有 AI 模型/开发者必读）

开始任何编码任务之前，先完整阅读 `docs/PLAN.md`，并严格按其中的
Phase 顺序和"铁律"执行。不在 PLAN.md 中的任务不要做。

最重要的三条规则：

1. 真 tmux 是唯一行为标准，不确定就实际运行 tmux 观察，不要猜。
2. 测试先行：先在 `scripts/compat_probe.sh` 或 Go 测试写好预期，再实现。
3. 每个任务结束必须全部通过：
   `go build ./... && go test ./... && scripts/compat_probe.sh`，
   然后更新 `docs/COMPATIBILITY.md`。
