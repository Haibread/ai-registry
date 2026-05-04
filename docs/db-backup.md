# Database backup & restore

The chart manages Postgres via the **CloudNativePG (CNPG) operator**. CNPG
uses [Barman](https://pgbarman.org/) to ship continuous WAL + scheduled base
backups to object storage. This document captures the recommended setup,
the current (unopinionated) default, and the restore playbook.

## Current default

**There is no backup enabled in the default chart values.** `cnpg.enabled`
is `false`; when turned on, a single-instance cluster is created with no
`backup` stanza. That is safe only for development.

## Objectives

- **RPO (recovery point objective):** ≤ 5 minutes — achievable via continuous
  WAL archiving with a 5-minute archive timeout.
- **RTO (recovery time objective):** ≤ 15 minutes for an intra-region PITR
  restore of a cluster under 20 GB. Larger datasets scale linearly with
  object-storage throughput.
- **Retention:** 30 days of base backups + WAL. Longer retention is a
  compliance decision, not a technical one.

## Enabling backups in the chart

Add the following to `deploy/helm/ai-registry/templates/cnpg-cluster.yaml`
(or fork the chart with an override) and wire the credentials via a
pre-created Kubernetes Secret.

```yaml
spec:
  # … existing fields …

  backup:
    barmanObjectStore:
      destinationPath: s3://my-backups/ai-registry
      endpointURL: https://s3.eu-west-1.amazonaws.com
      s3Credentials:
        accessKeyId:
          name: ai-registry-backup-creds
          key: ACCESS_KEY_ID
        secretAccessKey:
          name: ai-registry-backup-creds
          key: SECRET_ACCESS_KEY
      wal:
        compression: gzip
        maxParallel: 8
      data:
        compression: gzip
        immediateCheckpoint: false
        jobs: 2
    retentionPolicy: "30d"
```

Also create a `ScheduledBackup`:

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: ScheduledBackup
metadata:
  name: ai-registry-daily
spec:
  schedule: "0 4 * * *"    # 04:00 UTC daily
  backupOwnerReference: self
  cluster:
    name: ai-registry-postgres
```

Secret template (apply separately; **do not** commit real credentials):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ai-registry-backup-creds
type: Opaque
stringData:
  ACCESS_KEY_ID: "…"
  SECRET_ACCESS_KEY: "…"
```

## Verifying a backup

```sh
# List backups
kubectl cnpg backup list -n <ns> <cluster>

# Inspect a specific backup
kubectl describe backup <backup-name>
```

CNPG exports Prometheus metrics for backup age and status — alert on
`cnpg_backup_last_successful_seconds` growing beyond one day.

## Restore / PITR

CNPG restores by creating a **new** cluster with `bootstrap.recovery`
pointing at the backed-up object store. You cannot restore into an existing
running cluster — that's by design.

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: ai-registry-postgres-restored
spec:
  instances: 1
  bootstrap:
    recovery:
      source: ai-registry-postgres-backup
      recoveryTarget:
        # Either a backup name, or a timestamp for PITR:
        targetTime: "2026-04-20 12:00:00"
  externalClusters:
    - name: ai-registry-postgres-backup
      barmanObjectStore:
        destinationPath: s3://my-backups/ai-registry
        s3Credentials:
          accessKeyId:    { name: ai-registry-backup-creds, key: ACCESS_KEY_ID }
          secretAccessKey: { name: ai-registry-backup-creds, key: SECRET_ACCESS_KEY }
  storage:
    size: 20Gi
```

Apply and wait for `Status: Cluster in healthy state`. Then:

1. Point the server's `DATABASE_URL` at the restored cluster's superuser
   secret.
2. Scale the server Deployment to 0 → 1 to flush connections.
3. Verify with `test/load/smoke.js`.

## Drill checklist (quarterly)

- [ ] Trigger an on-demand backup.
- [ ] Restore it into a scratch namespace.
- [ ] Run the smoke test against the restored cluster's service.
- [ ] Record: backup size, restore duration, any failures. File an issue
      with tags `runbook/drill`.

If any step fails, treat it as a P0 — your real disaster-recovery capability
is broken.
