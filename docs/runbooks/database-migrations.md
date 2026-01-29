# Database Migration Runbook

## Overview

This runbook covers database migration procedures for the Jobsity Chat application.

**Tool:** golang-migrate (https://github.com/golang-migrate/migrate)

---

## Pre-Migration Checklist

Before running ANY migration in production:

- [ ] Test migration in staging environment first
- [ ] Create database backup
- [ ] Verify backup is restorable
- [ ] Check disk space (migrations, backup, rollback space)
- [ ] Verify migration files exist (up AND down)
- [ ] Review migration SQL for dangerous operations (DROP, TRUNCATE)
- [ ] Schedule maintenance window if needed
- [ ] Notify team in Slack/communication channel
- [ ] Have rollback plan ready
- [ ] Check current migration version: `make migrate-version`

---

## Running Migrations

### Staging Environment

```bash
# 1. Check current version
make migrate-version

# 2. Apply all pending migrations
make migrate-up

# 3. Verify application works
curl http://staging.example.com/health/ready

# 4. Run integration tests
make test-integration
```

### Production Environment

```bash
# 1. Create backup
pg_dump -h $DB_HOST -U $DB_USER -d $DB_NAME -F c -b -v -f backup_$(date +%Y%m%d_%H%M%S).dump

# 2. Verify backup
pg_restore --list backup_YYYYMMDD_HHMMSS.dump | head -20

# 3. Check current version
make migrate-version

# 4. Apply migrations
make migrate-up

# 5. Verify health
curl https://api.example.com/health/ready

# 6. Monitor logs for errors
kubectl logs -f deployment/chat-server --tail=100
```

---

## Rollback Procedures

### Scenario 1: Migration Failed (Partially Applied)

**Symptoms:** Migration command failed, database in inconsistent state

**Steps:**

1. **Check migration status:**
   ```bash
   make migrate-version
   # Shows: dirty version (e.g., 3 dirty)
   ```

2. **Force version to previous:**
   ```bash
   # If migration 3 failed, force to version 2
   migrate -path migrations -database "$DATABASE_URL" force 2
   ```

3. **Verify database state:**
   ```bash
   psql $DATABASE_URL -c "\dt"  # List tables
   psql $DATABASE_URL -c "\d users"  # Describe specific table
   ```

4. **Manual cleanup if needed:**
   ```sql
   -- Review what the failed migration did
   -- Manually revert changes if necessary
   BEGIN;
   -- Your cleanup SQL here
   COMMIT;
   ```

5. **Fix migration file and retry:**
   ```bash
   # Fix the migration SQL
   make migrate-up
   ```

### Scenario 2: Migration Succeeded but App Broken

**Symptoms:** Migration completed, but application has errors

**Steps:**

1. **Rollback last migration:**
   ```bash
   make migrate-down
   ```

2. **Verify application recovers:**
   ```bash
   curl https://api.example.com/health/ready
   kubectl logs -f deployment/chat-server --tail=50
   ```

3. **If app still broken, restore from backup:**
   ```bash
   # Stop application
   kubectl scale deployment/chat-server --replicas=0

   # Drop and restore database
   dropdb -h $DB_HOST -U $DB_USER $DB_NAME
   createdb -h $DB_HOST -U $DB_USER $DB_NAME
   pg_restore -h $DB_HOST -U $DB_USER -d $DB_NAME backup_YYYYMMDD_HHMMSS.dump

   # Restart application
   kubectl scale deployment/chat-server --replicas=2
   ```

### Scenario 3: Need to Rollback Application + Database

**Symptoms:** New version deployed, need to revert everything

**Steps:**

1. **Rollback application first:**
   ```bash
   # Kubernetes
   kubectl rollout undo deployment/chat-server

   # Docker Compose
   git checkout <previous-commit>
   docker-compose up -d --build
   ```

2. **Check migration version mismatch:**
   ```bash
   # If app expects older schema
   make migrate-version  # Shows current: 5
   # But app code expects: 3
   ```

3. **Rollback migrations:**
   ```bash
   # Rollback 2 migrations (5 → 4 → 3)
   make migrate-down  # Once
   make migrate-down  # Twice

   # Or rollback to specific version
   migrate -path migrations -database "$DATABASE_URL" goto 3
   ```

4. **Verify versions match:**
   ```bash
   make migrate-version  # Should show: 3
   git log -1 --oneline   # Should be commit with migration 3
   ```

---

## Common Issues

### Issue: "dirty database version X"

**Cause:** Migration failed mid-execution

**Fix:**
```bash
# Force to previous clean version
migrate -path migrations -database "$DATABASE_URL" force X-1

# Manually fix database if needed
psql $DATABASE_URL

# Retry migration
make migrate-up
```

### Issue: "no change" when running migrate-down

**Cause:** No down migration file exists

**Fix:**
```bash
# Check if .down.sql file exists
ls migrations/*_*.down.sql

# If missing, manually write down migration
# Then run: make migrate-down
```

### Issue: Backup restore fails

**Cause:** Restore incompatible with backup format

**Fix:**
```bash
# For custom format backups (.dump)
pg_restore -h $DB_HOST -U $DB_USER -d $DB_NAME backup.dump

# For SQL format backups (.sql)
psql -h $DB_HOST -U $DB_USER -d $DB_NAME < backup.sql
```

---

## Recovery Procedures

### Complete Database Recovery

**When:** Database corrupted, need full restore

**Steps:**

1. **Stop all applications:**
   ```bash
   kubectl scale deployment/chat-server --replicas=0
   kubectl scale deployment/stock-bot --replicas=0
   ```

2. **Drop and recreate database:**
   ```bash
   dropdb -h $DB_HOST -U $DB_USER $DB_NAME
   createdb -h $DB_HOST -U $DB_USER $DB_NAME
   ```

3. **Restore from backup:**
   ```bash
   pg_restore -h $DB_HOST -U $DB_USER -d $DB_NAME --verbose backup_YYYYMMDD_HHMMSS.dump
   ```

4. **Verify schema version:**
   ```bash
   psql $DATABASE_URL -c "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1;"
   ```

5. **Restart applications:**
   ```bash
   kubectl scale deployment/chat-server --replicas=2
   kubectl scale deployment/stock-bot --replicas=1
   ```

6. **Verify health:**
   ```bash
   curl https://api.example.com/health/ready
   ```

---

## Backup Strategy

### Automated Backups

**Frequency:** Daily at 2 AM UTC

**Retention:**
- Daily backups: 7 days
- Weekly backups: 4 weeks
- Monthly backups: 12 months

**Location:** S3 bucket `s3://backups.example.com/jobsity-chat/`

### Manual Backup

```bash
# Before any risky operation
pg_dump -h $DB_HOST -U $DB_USER -d $DB_NAME \
  -F c -b -v \
  -f backup_$(date +%Y%m%d_%H%M%S).dump

# Compress for storage
gzip backup_YYYYMMDD_HHMMSS.dump

# Upload to S3 (optional)
aws s3 cp backup_YYYYMMDD_HHMMSS.dump.gz \
  s3://backups.example.com/jobsity-chat/manual/
```

---

## Migration Best Practices

### Writing Migrations

✅ **DO:**
- Make migrations idempotent when possible
- Add both up AND down migrations
- Test migrations in staging first
- Use transactions for multi-statement migrations
- Add indexes concurrently to avoid locks
- Validate data before constraints

❌ **DON'T:**
- Drop columns without checking dependencies
- Add NOT NULL columns without defaults
- Run long-running operations without maintenance window
- Forget to add down migrations
- Mix DDL and DML in same migration (separate them)

### Example Safe Migration

```sql
-- 000004_add_last_seen.up.sql
BEGIN;

-- Add column with default (safe)
ALTER TABLE users
ADD COLUMN last_seen TIMESTAMP DEFAULT NOW();

-- Create index concurrently (no table lock)
COMMIT;
CREATE INDEX CONCURRENTLY idx_users_last_seen ON users(last_seen);
```

```sql
-- 000004_add_last_seen.down.sql
DROP INDEX IF EXISTS idx_users_last_seen;
ALTER TABLE users DROP COLUMN IF EXISTS last_seen;
```

---

## Emergency Contacts

**On-Call Engineer:** Check PagerDuty rotation
**Database Team:** #database-help Slack channel
**DevOps Team:** #devops Slack channel

---

## Change Log

- 2026-01-29: Initial version
