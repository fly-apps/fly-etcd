# Fly Etcd

Etcd deployment built to run on Fly.

## Getting Started

### Initialize a New Fly App

```bash
fly launch
```

> **Note:** Before running the command above, ensure the mount destination within the `fly.toml` is set to the `/data` directory. If this is not set correctly, the deploy will fail.

## Horizontal Scaling

While technically possible to scale your Etcd app to multiple members simultaneously, it's recommended to scale in increments of one until you've reached your target cluster size.

### Adding Members

When scaling, monitor your logs for errors and ensure your cluster is healthy before performing any subsequent scaling operations.

```bash
fly machines clone <machine-id>
```

This clone command is preferred over `fly scale count N` as it enforces unique zones for volume placement. Newly provisioned members will automatically join an existing cluster.

### Replacing Members

In the event you need to replace a member, it's better to add the new member first before removing an old one.

> **Note:** This is not always possible given the number of unique zones within some regions.

1. **Identify the Member `id` and `name` of the member you want to remove.**

   SSH into one of the member machines and use these helper commands:

   ```bash
   # View endpoint status
   flyadmin endpoint status
   ```
   
   ```bash
   # List all members
   flyadmin member list
   ```

2. **If the Member is the leader, transfer leadership.**

   ```bash
   etcdctl move-leader <target-member-id>
   ```

3. **Stop the Machine (easier in a separate terminal session)**

   ```bash
   fly machine stop <machine-id>
   ```

4. **Remove the member from the cluster**

   ```bash
   flyadmin member remove <member-id>
   ```

## Backups and Restoration

### Enabling Backups

If the following secrets are set, automatic backups will be performed and uploaded to S3:

**Static credentials:**
```
AWS_SECRET_ACCESS_KEY
AWS_ACCESS_KEY_ID
AWS_REGION
```

**OIDC-based auth:**
```
AWS_ROLE_ARN
AWS_REGION
```

**Optional environment variables:**
```
S3_BUCKET (default: fly-etcd-backups)
BACKUP_INTERVAL (default: "1h")
```

### Listing Backups

```bash
flyadmin backup list
```

### Creating On-Demand Backup

```bash
flyadmin backup create
```

### Restoring from a Backup

1. **Scale cluster down to a single member**

   ```bash
   # Stop a non-leader Machine
   fly m stop <machine-id>

   # Remove the associated member from the cluster
   flyadmin member remove <member-id>
   ```

2. **Select which backup you'd like to restore**

   List the backups with `flyadmin backup list` and identify the ID/Version you'd like to restore from.

3. **Initiate the restore**

   > **WARNING: This will erase existing data**
   
   ```bash
   flyadmin b restore <backup-id>
   ```

4. **Restart the machine**

   ```bash
   fly m restart <machine-id>
   ```

5. **Scale Etcd cluster back up to 3 nodes**

   ```bash
   fly m clone <machine-id>
   ```

6. **Verify cluster status**

   ```bash
   flyadmin endpoint status
   ```
