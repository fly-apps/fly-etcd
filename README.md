# Fly Etcd

Etcd deployment built to run on Fly.

## Preparing your app

Initialize a new Fly app.

_Note: Specify 2379 as the internal port._ 

```
fly init
```


## Deploying a single node cluster


```
# Creates new volume named etcd_data. ( Required )
fly volumes create etcd_data --region ord --size 10

# Deploy your app.
fly deploy .
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

```bash
fly scale count 2

fly scale count 3
```


## Monitoring

## Administration

## Recovering from Quorum loss
