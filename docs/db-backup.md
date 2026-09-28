# Database backup & restore

The registry's only state is its PostgreSQL database. On Kubernetes it runs
under the **CloudNativePG (CNPG) operator**, which uses
[Barman](https://pgbarman.org/) to ship continuous WAL and scheduled base
backups to object storage and restores them by point-in-time recovery (PITR).

## Objectives

- **RPO:** ≤ 5 minutes, from continuous WAL archiving with CNPG's default
  `archive_timeout` of 5 minutes.
- **RTO:** ≤ 15 minutes for an intra-region PITR restore of a cluster under
  20 GB; larger datasets scale with object-storage throughput.
- **Retention:** 30 days of base backups and WAL. Longer retention is a
  compliance decision, not a technical one.

## Where backups are configured

The CNPG `Cluster` the chart renders with `cnpg.enabled: true`
([templates/cnpg-cluster.yaml](../deploy/helm/ai-registry/templates/cnpg-cluster.yaml))
has no `backup` stanza, and the chart exposes no values for one. A backed-up
database is therefore a `Cluster` managed outside the chart, which the server
reaches through `api.database.existingSecret`:

1. Leave `cnpg.enabled` at `false`.
2. Create the `Cluster` and its `ScheduledBackup` (below) in the release
   namespace.
3. Create a Secret holding the DSN under the key `DATABASE_URL`. The `uri` key
   of the `<cluster>-app` Secret CNPG generates targets the application
   database and works as is:

   ```sh
   kubectl -n <ns> create secret generic ai-registry-database \
     --from-literal=DATABASE_URL="$(kubectl -n <ns> get secret ai-registry-postgres-app -o jsonpath='{.data.uri}' | base64 -d)"
   ```

4. Set `api.database.existingSecret: ai-registry-database`.

The application role owns the database, which is all the server's migrations
need.

## Configuring backups

Object-storage credentials live in a Secret created out of band (external
secrets, sealed secrets, or by hand — never committed):

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

The `Cluster`, with the same shape as the chart's plus a `backup` stanza:

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: ai-registry-postgres
spec:
  instances: 1
  imageName: ghcr.io/cloudnative-pg/postgresql:18
  bootstrap:
    initdb:
      database: ai_registry
      owner: ai_registry
  storage:
    size: 5Gi
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

A daily base backup:

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: ScheduledBackup
metadata:
  name: ai-registry-daily
spec:
  schedule: "0 0 4 * * *"    # 04:00 UTC daily (CNPG cron has a seconds field)
  backupOwnerReference: self
  cluster:
    name: ai-registry-postgres
```

The in-tree `barmanObjectStore` is deprecated upstream in favour of the
[Barman Cloud plugin](https://cloudnative-pg.io/plugin-barman-cloud/); follow
the [CNPG backup documentation](https://cloudnative-pg.io/documentation/current/backup/)
for the operator version you run.

## Verifying a backup

```sh
kubectl cnpg backup list -n <ns> ai-registry-postgres
kubectl -n <ns> describe backup <backup-name>
```

CNPG exports Prometheus metrics for backups: alert when
`cnpg_collector_last_available_backup_timestamp` is more than a day old, and on
any `cnpg_collector_last_failed_backup_timestamp` more recent than it.

## Restore / PITR

CNPG restores by creating a **new** cluster bootstrapped from the object store;
it never restores into a running cluster.

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: ai-registry-postgres-restored
spec:
  instances: 1
  imageName: ghcr.io/cloudnative-pg/postgresql:18
  bootstrap:
    recovery:
      source: origin
      # Omit recoveryTarget to replay all archived WAL.
      recoveryTarget:
        targetTime: "2026-04-20 12:00:00+00"
  externalClusters:
    - name: origin
      barmanObjectStore:
        destinationPath: s3://my-backups/ai-registry
        # The backed-up cluster's name: backups live under
        # <destinationPath>/<serverName>.
        serverName: ai-registry-postgres
        s3Credentials:
          accessKeyId: { name: ai-registry-backup-creds, key: ACCESS_KEY_ID }
          secretAccessKey: { name: ai-registry-backup-creds, key: SECRET_ACCESS_KEY }
  storage:
    size: 5Gi
```

If the restored cluster gets its own `backup` stanza, it must archive to a
different `destinationPath` or `serverName`: CNPG refuses to archive into a
non-empty WAL archive.

Once `kubectl cnpg status -n <ns> ai-registry-postgres-restored` reports a
healthy cluster:

1. Point the `DATABASE_URL` Secret at the restored cluster's
   `ai-registry-postgres-restored-app` `uri` (same command as above).
2. Restart the server so the pool reconnects:
   `kubectl -n <ns> rollout restart deploy/<fullname>-api`.
3. Run the smoke test against the deployment
   ([test/load/README.md](../test/load/README.md)).

## Restore drill

Run it quarterly — a backup that has never been restored is not a backup:

1. Trigger an on-demand backup:
   `kubectl cnpg backup -n <ns> ai-registry-postgres`.
2. Restore it into a scratch namespace with the manifest above.
3. Run the smoke test against a server pointed at the restored cluster.
4. Record the backup size, the restore duration and any failure.

A failed drill means disaster recovery is broken: treat it as an incident.
