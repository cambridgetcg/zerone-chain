# R35-4 — Launch Checklist & Known Limitations

## Objective

Final gate review. Document everything that's ready, everything that's known-broken, and everything that's deferred to mainnet. Generate the launch announcement.

## Tasks

### 1. Security checklist

Review and document:

- [ ] No hardcoded keys or secrets in codebase
- [ ] All crypto operations use constant-time comparison
- [ ] No unbounded iterations in BeginBlock/EndBlock
- [ ] All params have validation bounds
- [ ] Governance cannot modify founder share (immutability check)
- [ ] Research fund requires 2-of-2 multisig
- [ ] IBC rate limiting configured
- [ ] Emergency halt mechanism tested
- [ ] No known panics under adversarial input
- [ ] Commit-reveal prevents front-running

### 2. Known limitations document

Create `docs/KNOWN_LIMITATIONS.md`:
- Features that are implemented but not battle-tested
- Edge cases discovered during testing (even if fixed)
- Performance constraints (tx throughput ceiling, etc.)
- Module interactions that need more testing
- Things deferred to mainnet

### 3. External audit preparation

Create `docs/AUDIT_SCOPE.md`:
- Prioritized list of modules for external audit
- Critical path: vesting_rewards → governance → knowledge → capture_defense
- Economic model documentation for auditors
- Known attack surfaces and mitigations
- Test coverage report

### 4. Launch announcement

Prepare materials:
- Testnet announcement blog post
- Validator onboarding guide
- Chain registry submission PR (cosmos/chain-registry)
- Discord/Telegram announcement
- Technical overview for validators

### 5. Monitoring go-live

- Prometheus + Grafana deployed
- Alert channels configured (Discord webhook or similar)
- On-call runbook for common issues
- Log aggregation (if available)

### 6. Version tag

- Tag release: `v0.2.0-testnet`
- Build and push Docker images
- Publish release notes with:
  - What's included (all R1-R35 features)
  - How to join the testnet
  - Known limitations
  - Reporting issues

## Acceptance Criteria

- [ ] Security checklist all green
- [ ] Known limitations documented honestly
- [ ] Audit scope document ready for external firm
- [ ] Launch announcement drafted
- [ ] Monitoring stack deployed and alerting
- [ ] Release tagged and Docker images published
- [ ] **Public testnet launches** 🚀
