# K8s-bench Evaluation Results

## Model Performance Summary

| Agent | Success | Fail |
|-------|---------|------|
| gemini-cli (gemini-2.5-flash) | 2 | 1 |
| gemini-cli (gemini-2.5-pro) | 3 | 0 |
| **Total** | 5 | 1 |

## Overall Summary

- Total Runs: 6
- Overall Success: 5 (83%)
- Overall Fail: 1 (17%)

## Agent: gemini-cli

### Configuration: gemini-cli (gemini-2.5-flash)

| Task | Provider | Tool Use Shim | Result |
|------|----------|---------------|--------|
| create-pod | gemini | disabled | ✅ success |
| fix-crashloop | gemini | disabled | ❌ fail |
| scale-deployment | gemini | disabled | ✅ success |

**gemini-cli (gemini-2.5-flash) Summary**

- Total: 3
- Success: 2 (67%)
- Fail: 1 (33%)

### Configuration: gemini-cli (gemini-2.5-pro)

| Task | Provider | Tool Use Shim | Result |
|------|----------|---------------|--------|
| create-pod | gemini | disabled | ✅ success |
| fix-crashloop | gemini | disabled | ✅ success |
| scale-deployment | gemini | disabled | ✅ success |

**gemini-cli (gemini-2.5-pro) Summary**

- Total: 3
- Success: 3 (100%)
- Fail: 0 (0%)

---

_Report generated on September 11, 2025 at 1:02 PM_
