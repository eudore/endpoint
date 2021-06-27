# endpoint

endpoint是eudore集成gorm、opentracing、prometheus等第三方库扩展，可以参考endpoint发挥eudore的扩展能力，自定义一套体系。

| 组件  | 功能  |  状态 |  描述 |
| ------------ | ------------ | ------------ | ------------ |
| jaeger  | 全链路日志  | 使用  | 纯go实现<br>组件功能齐全  |
| preometheus | 监控采集 | 使用 | 自定义数据上报<br>与k8s/endpoints集成服务发现 |
| gorm/v2  | 数据库管理  | 使用  | 易于扩展<br>v2新版本<br>AutoMigrate<br>基本orm功能  |
| k8s/env configmaps  | 配置管理  | 可用  | k8s集成 |
| remote-service  | 配置管理  | 可用  | 自定义远程服务读取  |
| redis  | 缓存  | 可用  | 不熟悉  |
| k8s/svc  | 服务发现  | 使用  |  熟悉<br>与ci/cd集成<br>代码无关 |
| etcd  | 服务发现  | 不考虑  | 源码管理混乱  |
| consul  |  服务发现 | 观望  | 不熟悉  |
| consul  | 分布式锁  | 观望  |   |
| redis  | 分布式锁  | 观望  |   |
| eudore/policy | 权限 | 使用 | 细粒度pbac<br>数据权限 |
| k8s+gitlab | ci/cd | 后续开源 | |


# Example

[example](_example/app.go)运行依赖postgres、jaegertracing、prometheus服务，运行下面docker命令启动服务，命令仅为演示使用仅启动服务端口，不进行持久化等配置。

```bash
docker run -d -p 5432:5432 -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=postgres library/postgres:10.5
docker run -d -p 6831:6831/udp -p 16686:16686 jaegertracing/all-in-one:latest
docker run -d -p 9090:9090 prom/prometheus
```
