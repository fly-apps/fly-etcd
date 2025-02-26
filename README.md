# Fly Etcd

Etcd deployment built to run on Fly.

## Preparing your app

### Initialize a new Fly App

_Note: Before running the command below, ensure the mount destination within the `fly.toml` is set to the `/data` directory.  If this is NOT set correctly the deploy will fail._
```
fly launch
```



## Horizontal scaling

*Note: While you can *technically* scale your Etcd app up multiple members at a time, it's recommended that you scale in increments of one until you've reached your target cluster size.*

### Adding members
When scaling, make sure you monitor your logs for errors and ensure your cluster is healthy before performing any subsequent scaling operations.

```
fly machines clone <machine-id>
```

This clone command is preferrred over `fly scale count N` as it enforces unique zones for volume placement.  Newly provisioned members will automatically join an existing cluster.

### Replacing members
In the event you need to replace a member, it's always better to add the new member first before removing an old member.

_Note: This is not always possible given the number of unique zones within some regions._

**1. Identify the Member `id` and `name` of the member you are looking to remove.**

SSH into one of the member machines and from there you will have a couple helper commands you can leverage:

```
root@17816955c12958:/# flyadmin endpoint status
+-----------------------------------------------------------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+
|                            ENDPOINT                             |        ID        | VERSION | DB SIZE | IS LEADER | IS LEARNER | RAFT TERM | RAFT INDEX | RAFT APPLIED INDEX | ERRORS |
+-----------------------------------------------------------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+
| http://4d89d600b32628.vm.fks-fly-pg-pg-qmx-1-etcd.internal:2379 |  246a9e512b30e8f |  3.5.16 |   22 MB |     false |      false |         8 |     472268 |             472268 |        |
| http://3d8d73ddb40e18.vm.fks-fly-pg-pg-qmx-1-etcd.internal:2379 | c919583d728770d8 |  3.5.16 |   22 MB |     false |      false |         8 |     472268 |             472268 |        |
| http://17816955c12958.vm.fks-fly-pg-pg-qmx-1-etcd.internal:2379 | fe893594d308d1b6 |  3.5.16 |   22 MB |      true |      false |         8 |     472268 |             472268 |        |
+-----------------------------------------------------------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+
```

```
root@17816955c12958:/# flyadmin member list
+------------------+---------+----------------+-----------------------------------------------------------------+-----------------------------------------------------------------+------------+
|        ID        | STATUS  |      NAME      |                           PEER ADDRS                            |                          CLIENT ADDRS                           | IS LEARNER |
+------------------+---------+----------------+-----------------------------------------------------------------+-----------------------------------------------------------------+------------+
|  246a9e512b30e8f | started | 4d89d600b32628 | http://4d89d600b32628.vm.fks-fly-pg-pg-qmx-1-etcd.internal:2380 | http://4d89d600b32628.vm.fks-fly-pg-pg-qmx-1-etcd.internal:2379 |      false |
| c919583d728770d8 | started | 3d8d73ddb40e18 | http://3d8d73ddb40e18.vm.fks-fly-pg-pg-qmx-1-etcd.internal:2380 | http://3d8d73ddb40e18.vm.fks-fly-pg-pg-qmx-1-etcd.internal:2379 |      false |
| fe893594d308d1b6 | started | 17816955c12958 | http://17816955c12958.vm.fks-fly-pg-pg-qmx-1-etcd.internal:2380 | http://17816955c12958.vm.fks-fly-pg-pg-qmx-1-etcd.internal:2379 |      false |
+------------------+---------+----------------+-----------------------------------------------------------------+-----------------------------------------------------------------+------------+
```


**2. If the Member happens to hold leaderer, transfer that leadership.**

```
etcdctl move-leader <target-member-id>
```

Assuming this command went through ok, you can move onto the next step.


**3. Stop the Machine ( easier if you do this within a separate terminal session)**

Techincally, we don't need to stop the Machine first but we also don't want any connections being routed to the Machine after it leaves the cluster.
That being the case, I recommend stopping the Machine first

```
fly machine stop <machine-id>
```

**4. Remove the member from the cluster**

```
flyadmin member remove <member-id>
```

**5. You're done!**


## Monitoring

## Recovering from Quorum loss


## Backups and Restoring from backups

### Enabling backups
If the following environment variables are set, a backup will be performed and uploaded to S3 on a schedule:

**Static credentials:**
```
AWS_SECRET_ACCESS_KEY
AWS_ACCESS_KEY_ID
AWS_REGION
```

**OIDC based-auth:**
```
AWS_ROLE_ARN
AWS_REGION
```

Optional environment variables:
```
S3_BUCKET (default: fly-etcd-backups)
BACKUP_INTERVAL (default: "1h")
```

### Listing backups
```
root@148e6d1a149508:/data# flyadmin backup list
+----------------------------------+----------------------+-------+--------+
|                ID                |    LAST MODIFIED     | SIZE  | LATEST |
+----------------------------------+----------------------+-------+--------+
| cgPHQidz89s6_.0AOxymGy7PPU9CPRe5 | 2025-02-25T16:11:44Z | 29 kB |   true |
| .Nyq9HkyaqmTa0KXz_YcUNBcUtsOIsB2 | 2025-02-25T16:09:20Z | 29 kB |  false |
| JTP41hf8vKNTDFZKjbv_G2WoBO.p0XRq | 2025-02-25T01:36:04Z | 29 kB |  false |
+----------------------------------+----------------------+-------+--------+
```

### Creating on-demand backup
```
root@148e6d1a149508:/# flyadmin backup create
{"level":"info","ts":"2025-02-25T17:42:40.435283Z","logger":"etcd-client","caller":"v3@v3.5.18/maintenance.go:212","msg":"opened snapshot stream; downloading"}
{"level":"info","ts":"2025-02-25T17:42:40.439366Z","logger":"etcd-client","caller":"v3@v3.5.18/maintenance.go:220","msg":"completed snapshot read; closing"}
Backup created: /tmp/etcd-manual-backup2660955671/backup-20250225-174240.db (29 kB)
Backup uploaded to S3 as version: KK_iJNSYrBzdTzNFZByMICrUcFKo9j8o
```

### Restoring from a backup

A backup can be restored using the following process:

**1. Scale down Etcd to a single node**
Specific instructions for this is coming soon, but the "replacing" machines instructions is close.

**2. Select which backup you'd like to restore**
List the backups and identify the ID of the you'd like to restore.

**3. Initiate the restore.**
**WARNING: This will blow away existing data**
```
root@148e6d1a149508:/# flyadmin b restore KK_iJNSYrBzdTzNFZByMICrUcFKo9j8o
+----------+----------+------------+------------+
|   HASH   | REVISION | TOTAL KEYS | TOTAL SIZE |
+----------+----------+------------+------------+
| 7814136d |        3 |         10 |      29 kB |
+----------+----------+------------+------------+
Backup KK_iJNSYrBzdTzNFZByMICrUcFKo9j8o downloaded and saved to /tmp/etcd-restore-674497690/backup-restore.db
2025/02/25 17:46:54 Found etcd process (PID 650) with args: [etcd --config-file /data/etcd.yaml ]
Deprecated: Use `etcdutl snapshot restore` instead.

2025-02-25T17:46:56Z	info	snapshot/v3_snapshot.go:265	restoring snapshot	{"path": "/tmp/etcd-restore-674497690/backup-restore.db", "wal-dir": "/data/member/wal", "data-dir": "/data", "snap-dir": "/data/member/snap", "initial-memory-map-size": 0}
2025-02-25T17:46:56Z	info	membership/store.go:141	Trimming membership information from the backend...
2025-02-25T17:46:56Z	info	membership/cluster.go:421	added member	{"cluster-id": "b14f8c3fd657c435", "local-member-id": "0", "added-peer-id": "36f3ddaf5915706b", "added-peer-peer-urls": ["http://148e6d1a149508.vm.shaun-etcd-test.internal:2380"]}
2025-02-25T17:46:56Z	info	snapshot/v3_snapshot.go:293	restored snapshot	{"path": "/tmp/etcd-restore-674497690/backup-restore.db", "wal-dir": "/data/member/wal", "data-dir": "/data", "snap-dir": "/data/member/snap", "initial-memory-map-size": 0}
```

**4. Restart the machine**
```
fly m restart <machine-id>
```

**5. Scale Etcd cluster back up to 3 nodes**
See horizontal scaling instructions.
