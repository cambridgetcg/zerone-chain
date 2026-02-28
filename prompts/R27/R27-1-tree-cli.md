# R27-1 — Complete Tree Module CLI

## Context

The tree module has **29 message types** but only **4 tx CLI commands** (create-project, add-task, deploy-service, call-service) and **6 query commands**. That's 25 missing tx commands. Users can't manage the project lifecycle, task workflow, service operations, or seeding from the command line.

The R25 assessment scored tree CLI at 2-5/10 across its subsystems, largely because operators couldn't exercise most features.

## Existing CLI

**Tx (4 of 29):**
- `create-project [name] [description]`
- `add-task [project-id] [title] [description] [bounty-amount]`
- `deploy-service [project-id] [name] [description] [endpoint] [price-per-call]`
- `call-service [service-id] [input-data] [payment-type]`

**Query (6):**
- `project [id]`, `projects-by-founder [addr]`, `task [id]`, `service [id]`, `seed [id]`, `params`

## Missing Tx Commands (25)

### Project Lifecycle (7)
- `propose-project [project-id]` — MsgProposeProject
- `start-development [project-id]` — MsgStartDevelopment (authority)
- `complete-project [project-id]` — MsgCompleteProject
- `pause-project [project-id]` — MsgPauseProject
- `resume-project [project-id]` — MsgResumeProject
- `abandon-project [project-id]` — MsgAbandonProject
- `spawn-child-project [parent-id] [title] [description]` — MsgSpawnChildProject

### Task Workflow (6)
- `assign-task [task-id] [assignee]` — MsgAssignTask
- `start-work [task-id]` — MsgStartWork
- `submit-deliverable [task-id] [content-hash] [description]` — MsgSubmitDeliverable
- `approve-deliverable [task-id]` — MsgApproveDeliverable
- `reject-deliverable [task-id] [reason]` — MsgRejectDeliverable
- `reopen-task [task-id]` — MsgReopenTask

### Contributor Management (3)
- `apply-to-project [project-id] [capabilities...]` — MsgApplyToProject
- `review-application [project-id] [applicant] [approved]` — MsgReviewApplication
- `add-contributor [project-id] [address] [role]` — MsgAddContributor

### Availability (1)
- `set-availability [available] [capacity] [capabilities...]` — MsgSetAvailability

### Service Operations (4)
- `subscribe-service [service-id] [duration]` — MsgSubscribeService
- `pause-service [service-id]` — MsgPauseService
- `resume-service [service-id]` — MsgResumeService
- `retire-service [service-id]` — MsgRetireService

### Seeding (3)
- `detect-opportunity [domain] [type] [description]` — MsgDetectOpportunity
- `begin-seeding [opportunity-id]` — MsgBeginSeeding
- `claim-opportunity [opportunity-id]` — MsgClaimOpportunity

### Admin (1)
- `update-params` — MsgUpdateParams (authority only)

## Task

### 1. Implement All Missing Tx Commands

Follow the pattern established by existing commands in `x/tree/client/cli/tx.go`. Each command:
- Parses args from command line
- Constructs the appropriate Msg
- Sets the `--from` signer
- Calls `tx.GenerateOrBroadcastTxCLI`

Group by priority — project lifecycle and task workflow first (most used), seeding last.

### 2. Fix deploy-service Off-by-One

From R25 assessment: "Service not linked to project" and "price dropped" — the CLI arg mapping skips the price argument. Verify the arg indices in the deploy-service command match the proto fields.

### 3. Add Missing Query Commands

Check if any query RPCs lack CLI exposure:
```bash
grep "rpc Query" proto/zerone/tree/v1/query.proto
```

Add CLI commands for any missing queries (e.g., list all tasks by project, list services by project, list seeds by domain).

### 4. Test Each Command

For each new command, verify:
- `--help` shows correct usage
- Valid invocation produces correct tx
- Missing required args produce helpful errors
- The tx succeeds on localnet

Don't need full E2E lifecycle tests for every command (R27-3 covers that), but each command should at least build and broadcast successfully.

## Files to Modify

- `x/tree/client/cli/tx.go` — Add 25 new tx commands
- `x/tree/client/cli/query.go` — Add any missing query commands
- Tests: at minimum, verify commands are registered and `--help` works

## Success Criteria

- [ ] All 29 message types have corresponding CLI commands
- [ ] deploy-service off-by-one fixed
- [ ] All queries exposed via CLI
- [ ] Each command works on localnet (tx broadcasts successfully)
- [ ] `zeroned tx tree --help` shows all subcommands
- [ ] All existing tests pass
