# Fly Etcd

Etcd deployment built to run on Fly.

## Preparing your app

**Initialize a new Fly App**

_Note: Client requests should be directed to port 2379._ 

```
fly launch
```

## Horizontal scaling

*Note: While you can *technically* scale your Etcd app up multiple members at a time, it's recommended that you scale in increments of one until you've reached your target cluster size.*


**Add additional volumes in your target region(s)**

```bash
$ fly volumes create etcd_data --region iad --size 10
$ fly volumes create etcd_data --region ewr --size 10
```

**Scale your app**

When scaling, make sure you monitor your logs for errors and ensure your cluster is healthy before performing any subsequent scaling operations.  Newly added members are considered for quorum even if the member is not reachable from other existing members.

```
fly scale count 2
```
```
fly scale count 3
```

## Administration



## Monitoring



## Recovering from Quorum loss


## Backups and Restoring from backups

If the following environment variables are set, a backup will be performed and uploaded to S3 on a schedule:

```
AWS_REGION
AWS_SECRET_ACCESS_KEY
AWS_ACCESS_KEY_ID
BACKUP_INTERVAL (default: "1h")
S3_BUCKET=fly-etcd-backups
```

A backup can be restored using the following process:

Scale down the app to a single node
On the remaining node, run `etcd-restore`. You can list available versions with `etcd-restore -list` and restore a specific version with `etcd-restore -version=<version>`.
`fly app restart`
Check the logs to make sure it's started and healthy.
Scale back up again one node at a time.
** You may have to delete the old volumes from the previous machines as they contain member config. Either that or delete the contents of `/data` before scaling down. **
