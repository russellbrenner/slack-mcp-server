# Security Audit and Remediation Plan (2026-05-12)

## Part 1 — Secure Code Upgrade

### Implemented in this PR

#### Finding SC-01: Sensitive data could be written to local disk (`pkg/provider/edge/edge.go`)
- **Risk:** `NewWithClient` always created `tape.txt` and routed request/response bodies through a recorder. Edge API requests include Slack tokens in form/json payloads, so this could persist credentials to disk.
- **Threat model:** local credential disclosure, accidental commit/artifact exfiltration, host compromise blast-radius increase.
- **Fix applied:**
  - Removed default `tape.txt` creation.
  - Made request/response recording opt-in only via `WithTape(...)`.
  - Ensured `NewWithClient(..., opt...)` now actually applies options.
- **Validation added:** new tests in `pkg/provider/edge/edge_security_test.go` verifying:
  - no `tape.txt` is created by default;
  - explicit `WithTape` still works.

### Additional code-level security observations (not all remediated in this PR)
- **SC-02: Optional insecure TLS mode** via `SLACK_MCP_SERVER_CA_INSECURE` (`pkg/transport/transport.go`).
  - Useful for debugging but unsafe in production (MITM risk).
  - Recommend production guardrails (explicit environment confirmation + startup warning/fail policy).
- **SC-03: HTTP/SSE can run without API key** (`pkg/server/auth/sse_auth.go`).
  - Current behavior is permissive by default.
  - Recommend forcing auth when binding non-loopback interfaces.

---

## Part 2 — Dependency Audit and Supply-Chain Intelligence

### Direct dependency vulnerability scan
- **Scanner used:** GitHub Advisory DB (`gh-advisory-database`) against key direct and high-impact transitive Go modules.
- **Result:** No known advisories were returned for the scanned versions at time of audit.
- **Packages checked:**  
  `github.com/mark3labs/mcp-go@0.44.0`, `github.com/openai/openai-go@1.12.0`, `github.com/refraction-networking/utls@1.8.2`, `github.com/rusq/slackauth@0.7.1`, `github.com/rusq/slackdump/v3@3.1.13`, `github.com/slack-go/slack@0.17.3`, `go.uber.org/zap@1.27.1`, `golang.org/x/net@0.50.0`, `golang.org/x/crypto@0.48.0`, `golang.org/x/text@0.34.0`, `golang.org/x/sys@0.41.0`, `github.com/klauspost/compress@1.18.0`, `google.golang.org/protobuf@1.36.6`.

### Supply-chain risk review
- **SCM provenance risk:** several dependencies are from less-common maintainers (not necessarily malicious, but higher review burden).
- **Build/runtime artifact risk:**
  - Docker image tags are mutable (`golang:1.24`, `alpine:3.22`) and not digest-pinned.
  - Consider digest pinning to reduce image drift and replay uncertainty.
- **Workflow surface risk:**
  - CI should keep least-privilege permissions and avoid exposing secrets to untrusted contexts.
- **Release integrity gaps to close:**
  - add SBOM + signed release artifacts + provenance attestations (SLSA-style).

---

## Self-executable hardening pipeline (parallelizable, conflict-safe PR tracks)

### Gate 0 — Reproducible security baseline (must pass before remediation PRs)
1. Clone repository and checkout target branch.
2. Run:
   - `make deps`
   - `make test`
   - `go test -count=1 ./...` (integration failures expected without required tokens; record as environmental)
3. Generate dependency inventory:
   - `go list -m all`
4. Record baseline artifacts:
   - test logs,
   - dependency list,
   - current workflow and Docker digests/tags.
5. **Gate:** baseline report committed.

### Track A (PR-A, code hardening) — secrets and auth boundaries
- Scope files: `pkg/provider/edge/*`, `pkg/server/auth/*`, `pkg/server/*`.
- Work:
  1. Keep SC-01 fix (this PR).
  2. Add non-loopback auth enforcement policy for HTTP/SSE.
  3. Add explicit warnings/fail-fast for insecure TLS mode in production.
- Tests:
  - unit tests for auth-required behavior on non-loopback bind;
  - unit tests proving secure defaults for transport options.
- **Gate:** no token material written to filesystem/logs in tests.

### Track B (PR-B, dependency and runtime hygiene) — no overlap with Track A files
- Scope files: `go.mod`, `go.sum`, `Dockerfile`, `.github/workflows/*`.
- Work:
  1. Upgrade dependencies in small batches (grouped by subsystem).
  2. Pin base images by digest.
  3. Keep Trivy/dependency scanning in CI blocking on high/critical.
- Tests:
  - `make test`
  - `go test -count=1 ./...`
  - CI security workflow execution.
- **Gate:** dependency diff + CVE delta included in PR description.

### Track C (PR-C, release integrity) — separate files from A/B where possible
- Scope files: release workflows and documentation.
- Work:
  1. Add SBOM generation.
  2. Add artifact signing + provenance attestation.
  3. Document verification workflow for downstream users/agents.
- Tests:
  - dry-run release workflow;
  - verify signature/provenance validation script in CI.
- **Gate:** verifier can validate produced artifacts from CI outputs.

### Runtime-realistic security test suite for agent execution
Use real transport and MCP surface, not synthetic mocks only:
1. Build artifact: `go build ./cmd/slack-mcp-server`.
2. Start server in SSE and HTTP modes with explicit bind/auth settings.
3. Configure agent harness using native MCP CLI/config first; fallback to config file wiring.
4. Execute MCP calls against live server:
   - read-only tool calls (`channels_list`, `users_search`) with valid auth;
   - negative tests with missing/invalid auth;
   - unsafe mode checks (insecure TLS/auth-open) expected to fail or warn per policy.
5. Capture:
   - request/response logs (redacted),
   - exit codes,
   - policy gate outcomes.
6. **Completion gate:** tests must execute live binary paths and assert expected runtime behavior, not only unit-level function contracts.

### Merge-order safety
- PR-A, PR-B, PR-C are designed to touch largely disjoint file sets.
- Any shared file (`.github/workflows/*`) should be isolated to one track at a time.
- If parallel tracks must touch same workflow, sequence as:
  1. merge PR-B workflow changes,
  2. rebase PR-C and keep only release/provenance deltas.

