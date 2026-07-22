# ForecastIQ — Secrets Management (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: NFR-SEC02/SEC12/SEC13; BR-08; threat model §4, §16

---

## 1. Secret Inventory

| Secret | Purpose | Storage (prod) | Storage (local dev) | Rotation | Blast radius |
|--------|---------|----------------|--------------------|---------|--------------|
| `DATABASE_URL` | App DB connection (app role) | systemd EnvironmentFile (`/etc/forecastiq/secrets.env`, 0600, root:root) | `.env.local` (gitignored) | 90 d (vendor credential rotation + env update + restart) | DB read/write as app role |
| `DATABASE_MIGRATE_URL` | DDL role for migrations | Same file | Same | With app credential | Schema changes |
| `OPENWEATHER_API_KEY` | Provider auth | Same file; referenced by `credential_ref = "OPENWEATHER_API_KEY"` | Same | On suspicion / 180 d | Provider account quota |
| `SUPABASE_SERVICE_ROLE_KEY` | Admin API (ban/delete users) | Same file | Same (optional) | Vendor dashboard | Full Supabase project (highest) |
| `SUPABASE_URL` / `SUPABASE_ANON_KEY` | Client config | Public by design (anon key is scoped, safe for browser) | Same | Vendor | Minimal (RLS-scoped anon) |
| Deploy SSH key | Pipeline → VPS | GitHub Actions secret; VPS authorized_keys (command-restricted) | n/a | 180 d | VPS deploy |
| B2 (offsite backup) credentials | rclone config | `/root/.config/rclone/rclone.conf` (0600) | n/a | 180 d | Backup bucket |
| Grafana Cloud API key | Agent remote-write | grafana-agent config (0600) | n/a | 180 d | Observability ingest |
| Cloudflare API token | DNS automation (Terraform) | GitHub Actions secret (CI-only) | n/a | 180 d; token scoped to DNS edit for zone only | DNS records |

**No other secrets exist by design** (JWT verification uses public JWKS; no signing keys held).

## 2. Storage Rules (binding)

1. Production secrets: EnvironmentFile loaded by systemd only; file mode 0600; owned by root; app process reads via environment (never file-parsing at runtime).
2. **Never**: in repository (gitleaks gate), in container images (Trivy secret scan), in logs (allowlist), in API responses (struct-level absence), in dashboard bundle (service-role grep CI check), in GitHub Actions output (masked).
3. Local dev: `.env.local` (gitignored, template `.env.example` with placeholders committed).
4. Offline backup of production secrets: encrypted (1Password or age-encrypted file) in operator's password manager + sealed physical copy (bus factor, R-05).

## 3. credential_ref Indirection (BR-08 implementation)

```text
provider_configurations.credential_ref = "OPENWEATHER_API_KEY"   ← env var NAME
collector resolves: os.Getenv(credential_ref) at call time
```

- DB stores only the name → DB compromise does not leak the key.
- API never returns credential_ref value semantics (field absent from serializers; admin sees "Configured"/"Not set" status only).
- Rotation = env file update + restart; no DB change needed (unless renaming).

## 4. Rotation Procedures

### 4.1 Provider key (≤ 30 min)
1. Generate new key in provider dashboard (old key stays active if supported).
2. Update `/etc/forecastiq/secrets.env`.
3. `systemctl restart forecastiq`.
4. `POST /admin/collections/trigger` one test collection → verify success.
5. Revoke old key in provider dashboard.
6. Audit: `provider.config_updated` logged automatically on next config touch; manual ops-log entry for env-only rotation.

### 4.2 DB credential (90 d)
1. Vendor console: create new credential (app role).
2. Update env file; restart; verify readyz.
3. Drop old credential.

### 4.3 Service-role key
Vendor dashboard rotation → env update → restart → verify admin user-list endpoint.

### 4.4 Compromise response
Treat as incident: rotate immediately (§4.1–4.3), review audit + provider usage for abuse window, log incident. If DATABASE_URL leaked: rotate credential + review pg audit log for anomalous access.

## 5. Least Privilege (NFR-SEC12)

| Role | Rights |
|------|--------|
| App role (`forecastiq_app`) | DML on forecastiq schema tables; no DDL; cannot ALTER triggers; EXECUTE on owned functions only |
| Migration role (`forecastiq_migrate`) | DDL on schema; used only by deploy pipeline (separate credential) |
| Trigger-owner role | Owns immutability trigger functions; not used by app or migrations routinely |
| Managed DB admin | Vendor console only; break-glass, operator-owned |

## 6. Verification (CI + periodic)

| Check | Frequency |
|-------|-----------|
| gitleaks full scan | Every push + weekly scheduled history scan |
| Trivy image secret scan | Every build |
| Dashboard bundle grep for service-role/anon misuse | Every frontend build |
| Env file permissions audit (VPS) | Monthly (ops checklist) |
| Rotation calendar review | Monthly (ops checklist item per secret) |

## 7. Cross-Reference

- Threat model §4/§16: `docs/security/02-threat-model.md`
- Data classification (Confidential tier): `docs/security/03-data-classification.md`
- Deployment secrets flow: `docs/architecture/06-deployment-architecture.md` §7
