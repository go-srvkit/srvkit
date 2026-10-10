# uuid

分布式ID生成


# Usage


### 雪花算法

  时间戳 | WorkerID | SeqID
--------|--------|-------

* 把一个64位的整数分成3个区间，分别用于`时间戳`、`workerID`和`seqID`；
* 时间戳代表时间单元需要一直递增，不同的时间单元实现有毫秒、秒、厘秒等，这里依赖时钟不回调；
* `workerID`用来标识分布式环境下的每个service；
* `seqID`在一个时间单元内持续自增，如果在单个时间单元内`seqID`溢出了，需要sleep等待进入下一个时间单元；

分布式部署下雪花ID只保证了唯一性，无法保证顺序性（递增），
因为ID的分段包含了服务ID，在同个时间单元内(10ms)服务A按时钟后生成的ID会小于服务B按时钟先生成的ID。

不同的雪花算法的参考实现。

* sony的实现 `https://github.com/sony/sonyflake`
* 百度的实现 `https://github.com/baidu/uid-generator`
* 美团的实现 `https://github.com/Meituan-Dianping/Leaf`

### 发号器算法

* 算法把一个64位的整数按step划分为N个号段；
* 每个service向发号器申请领取一个代表号段的counter；
* service内部使用这个号段向业务层分配ID；
* service重启或者号段分配完，向发号器申请下一个号段；
* 要求底层存储不能随意被修改影响到上层算法分配；

发号器依赖存储组件，对存储组件的需求是能实现整数自增。

发号器ID只保证了唯一性，无法保证顺序性（递增），因为多个服务同时生成， 如果服务1的生成速度如果比服务2快，服务1的ID号段会先用完，
那么服务1上按时钟先分配的ID会大于服务2上按时钟后分配的ID。

本包提供多种存储选择，redis、mongodb 和 etcd

* redis使用[incr](https://redis.io/commands/incr)命令实现自增
* mongodb使用[findOneAndUpdate](https://docs.mongodb.com/v4.4/reference/method/db.collection.findOneAndUpdate)
* etcd 每次独立执行一次 `Put`，使用该次响应的 `Header.Revision` 作为号段编号

### etcd 号段存储

`NewEtcdCounter(config clientv3.Config, key string)` 返回 `(*EtcdCounter, error)`。
配置可指定多个 endpoint、认证和 TLS；store 拥有所创建的客户端，使用完毕必须关闭，
包括 `Init` 失败的情况。`Init` 使用线性一致读检查访问权限与可用性，key 可以不存在，
不会领取号段。成功初始化后可重复调用；`Close` 可重复调用，关闭后不能重新初始化。
所有公共方法支持并发调用。关闭会取消未完成请求，已经开始的请求也可能成功完成。


* revision 是整个集群的修改版本号，其他业务写入会造成编号和 ID 的空洞，不影响唯一性。
* 相同 ID 空间必须使用同一集群、固定步长。多个独立集群的 revision 不能直接混用。
* 每次领取独立执行一次写入，不能把同一事务的 revision 分配给多个号段。
* 超时或断线时不使用该次领取结果；可能已经提交的写入只会浪费号段，不执行回滚。
* 正常 leader 切换保留已确认写入；失去多数派时不能领取新号段，本地已领取号段仍可使用。
* 从旧快照恢复、重建集群或切换存储后端前，必须保证新号段不覆盖历史已预留区间。
  快照恢复需要可靠的历史上界；`--bump-revision` 不能在没有上界依据时随意设置。
* 配置 etcd 的 MVCC 历史压缩，避免反复写入导致历史版本持续累积。

参考 [etcd API 保证](https://etcd.io/docs/v3.7/learning/api_guarantees/) 与
[灾难恢复](https://etcd.io/docs/v3.7/op-guide/recovery/)。
