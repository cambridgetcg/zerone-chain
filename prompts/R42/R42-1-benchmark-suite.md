# R42-1 — Benchmark Suite

## Objective

Build an automated benchmark suite that evaluates fine-tuned models across multiple domains, producing quantitative quality scores that feed the fitness feedback loop.

## Design

### Benchmark Structure

```
services/benchmark/
├── main.go              — CLI entry point
├── runner.go            — Benchmark execution engine
├── evaluator.go         — Answer evaluation (exact match, fuzzy, LLM-judge)
├── reporter.go          — Results aggregation and reporting
├── domains/
│   ├── code.go          — Code generation benchmarks
│   ├── reasoning.go     — Logical reasoning benchmarks
│   └── instruction.go   — Instruction following benchmarks
├── datasets/
│   ├── code_bench.json      — 50+ code generation test cases
│   ├── reasoning_bench.json — 30+ reasoning test cases
│   └── instruct_bench.json  — 20+ instruction following test cases
└── benchmark_test.go
```

### Benchmark Types

**Code Generation** (first vertical — highest priority):
- Function implementation from docstring
- Bug fixing (given broken code, fix it)
- Code review (identify issues in snippet)
- Test generation (write tests for given function)
- Evaluation: exact output match, test pass rate, syntax validity

**Reasoning**:
- Multi-step math problems
- Logic puzzles
- Cause-effect chains
- Evaluation: exact match, partial credit for intermediate steps

**Instruction Following**:
- Format compliance (JSON, markdown, specific structure)
- Constraint satisfaction (word count, style, tone)
- Multi-constraint tasks
- Evaluation: structural checks + LLM-judge scoring

### Evaluation Methods

1. **Exact match** — for deterministic answers (math, code output)
2. **Fuzzy match** — for semantic equivalence (Levenshtein, BLEU)
3. **Execution-based** — run generated code, check output (sandboxed)
4. **LLM-judge** — use base model to evaluate quality (for subjective tasks)

### Output Format

```json
{
  "model_version": "zerone-code-v1.2",
  "dataset_version": "tok-snapshot-1234",
  "timestamp": "2026-03-06T14:00:00Z",
  "overall_score": 0.73,
  "domain_scores": {
    "code": 0.81,
    "reasoning": 0.65,
    "instruction": 0.72
  },
  "per_case_results": [...]
}
```

### CLI

```bash
# Run full benchmark suite against an inference endpoint
zerone-bench run --endpoint http://localhost:8080/v1 --model zerone-code-v1.2

# Run specific domain
zerone-bench run --endpoint ... --domain code

# Compare two model versions
zerone-bench compare --baseline v1.1 --candidate v1.2
```

## Tests

- Test: runner executes all benchmarks and collects results
- Test: code evaluator correctly scores exact match / partial / fail
- Test: reporter aggregates per-domain and overall scores
- Test: comparison detects regressions

## Constraints

- Code execution in sandbox (Docker container or subprocess with timeout)
- Benchmark dataset is NOT part of training data (held out)
- Results stored in `services/benchmark/results/` as versioned JSON
