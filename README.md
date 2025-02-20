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

### Adding members
When scaling, make sure you monitor your logs for errors and ensure your cluster is healthy before performing any subsequent scaling operations.  Newly added members are considered for quorum even if the member is not reachable from other existing members.

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
