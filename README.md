timburr
=======

A MediaWiki event handler for [52Poké Wiki](https://wiki.52poke.com/). It subscribe to specific topics in Kafka, and execute an action in response to each event. Timburr is mainly designed to handle MediaWiki [job queue](https://www.mediawiki.org/wiki/Manual:Job_queue) and page cache purging.

Timburr works in a way similar to Wikimedia [Event Platform](https://wikitech.wikimedia.org/wiki/Event_Platform)'s [change propagation](https://github.com/wikimedia/mediawiki-services-change-propagation), but is more lightweight and appropriate for smaller wikis. It replaces both the [redis job runner](https://github.com/wikimedia/mediawiki-services-jobrunner) we previously used but no longer working after MediaWiki 1.33, and the cache purger [lilycove](https://github.com/mudkipme/lilycove).

## Pre-requisites

A [Kafka](https://kafka.apache.org/) cluster is required. A single-node Kafka cluster running in Docker should be good enough for small wikis like 52Poké Wiki.

```bash
docker create --name zookeeper --net isolated_nw \
    --restart=always \
    zookeeper

docker create --name kafka --net isolated_nw \
    -e KAFKA_HEAP_OPTS="-Xmx256M -Xms256M" \
    -e KAFKA_ZOOKEEPER_CONNECT=zookeeper:2181 \
    -e KAFKA_LISTENER_SECURITY_PROTOCOL_MAP=INSIDE:PLAINTEXT,OUTSIDE:PLAINTEXT \
    -e KAFKA_ADVERTISED_LISTENERS=INSIDE://:9092,OUTSIDE://127.0.0.1:9094 \
    -e KAFKA_LISTENERS=INSIDE://:9092,OUTSIDE://:9094 \
    -e KAFKA_INTER_BROKER_LISTENER_NAME=INSIDE \
    -e KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1 \
    -p 127.0.0.1:9094:9094 \
    -v kafka:/var/lib/kafka/data \
    --restart=always \
    confluentinc/cp-kafka
```

If [Logstash](https://www.elastic.co/cn/products/logstash) is used as an event producer, it needs to be installed too.

```bash
docker create --name logstash --net isolated_nw \
    -e LS_JAVA_OPTS="-Xmx256m -Xms256m" \
    -v <path-to-config>/pipelines.yml:/usr/share/logstash/config/pipelines.yml \
    -v <path-to-config>/pipeline_mediawiki.conf:/usr/share/logstash/pipelines/pipeline_mediawiki.conf \
    --restart=always \
    logstash:6.8.5
```

## Configuration

### Event Producer

An EventBus endpoint is needed to produce event to Kafka. A simple Logstash pipeline is used on 52Poké Wiki for this purpose.

```
input {
  http {
    codec => json { charset => "UTF-8" }
    port => 5000
  }
}

output {
  kafka {
    bootstrap_servers => "<kafka-broker>:9092"
    codec => json {
      charset => "UTF-8"
    }
    topic_id => "%{[meta][stream]}"
  }
}
```

On Wikimedia wikis, [EventGate](https://github.com/wikimedia/eventgate) is implemented as an event producer.

### MediaWiki

Timburr requires a [modified version of EventBus](https://github.com/mudkipme/mediawiki-extensions-EventBus) extension<sup>[1](#why-eventbus)</sup> and MediaWiki 1.35.

```php
$wgJobRunRate = 0;
$wgJobTypeConf['default'] = [ 'class' => '\\MediaWiki\\Extension\\EventBus\\Adapters\\JobQueue\\JobQueueEventBus', 'readOnlyReason' => false ];

wfLoadExtension( 'EventBus' );
$wgEnableEventBus = 'TYPE_ALL';
$wgEventServices = [
    'eventbus' => [ 'url' => 'http://<event-producer-host>:<event-producer-port>', 'timeout' => 10 ],
];

// only needed to handle cache purging
$wgEventRelayerConfig = [
    'cdn-url-purges' => [
        'class' => \MediaWiki\Extension\EventBus\Adapters\EventRelayer\CdnPurgeEventRelayer::class,
        'stream' => 'cdn-url-purges'
    ],
];

```

### Timburr

Write a `conf/config.yml` file with the following example:

```yaml
kafka:
  brokerList: "<kafka-broker>:9092"

options:
  groupIDPrefix: timburr-
  metadataWatchGroupID: timburr-watcher
  metadataWatchRefreshInterval: 10000
  logstash: "<logstash-server>:<logstash-port>" # the endpoint of Logstash tcp input, only needed to send logs to Logstash

jobRunner:
  endpoint: http://<mediawiki-host>/rest.php/eventbus/v0/internal/job/execute
  excludeFields: ["host", "headers", "@timestamp", "@version"] # exclude fields added by Logstash

purge: # only needed to handle cache purging
  expiry: 86400000  # cache expiry time, in milliseconds
  cfZoneID: <cloudflare zone id> # only needed if cloudflare CDN is used
  cfToken: <cloudflare token>
  entries:
  - host: <mediawiki-host> # entry for purging page cache
    method: PURGE # method to purge cache, see [libnginx-mod-http-cache-purge](https://packages.debian.org/buster/libnginx-mod-http-cache-purge) or [ngx_cache_purge](https://github.com/FRiCKLE/ngx_cache_purge) if nginx is used
    variants:
    - zh # only needed if cache keys contain language variants
    - zh-hans
    - zh-hant
    uris:
    - "http://<frontend-server>#url##variants#"
    - "http://<frontend-server>#url##variants#mobile" # only needed if cache keys differ between desktop and mobile
    headers:
      host: <mediawiki-host>
  - host: <image-host> # entries for purging image cache
    method: PURGE
    uris:
    - "http://<frontend-server>#url#"
    - "http://<frontend-server>/webp-cache#url#" # see [malasada](https://github.com/mudkipme/malasada)
    headers:
      host: <image-host>
  - host: <image-host> # entry for [malasada](https://github.com/mudkipme/malasada)
    method: DELETE
    uris:
    - "https://<api-gateway-endpoint>/<api-gateway-stage>/webp#url#"
  - host: <image-host> # only needed if cloudflare CDN is used
    method: Cloudflare
    uris:
    - "https://<cloudflare-host>#url#"

rules:
- name: basic
  topic: /^mediawiki\.job\./
  excludeTopics:
  - mediawiki.job.AssembleUploadChunks
  - mediawiki.job.PublishStashedFile
  - mediawiki.job.uploadFromUrl
  - mediawiki.job.cirrusSearchLinksUpdate
  - mediawiki.job.htmlCacheUpdate
  - mediawiki.job.refreshLinks

- name: low-priority
  topics:
  - mediawiki.job.cirrusSearchLinksUpdate
  - mediawiki.job.htmlCacheUpdate
  - mediawiki.job.refreshLinks

- name: upload
  topics:
  - mediawiki.job.AssembleUploadChunks
  - mediawiki.job.PublishStashedFile
  - mediawiki.job.uploadFromUrl

- name: purge # only needed to handle cache purging
  topic: cdn-url-purges
  taskType: purge
  rateLimit: 5 # only handle 5 events in 10000 milliseconds in this rule group
  rateInterval: 10000
```

## Installation

Golang and librdkafka-dev is required to compile timburr. It is recommended to run timburr via a [Docker image](https://github.com/users/mudkipme/packages/container/package/timburr).

```bash
docker create --name timburr --net isolated_nw \
  -v <path-to-config>/timburr.yml:/app/conf/config.yml \
  --restart=always \
  mudkip/timburr
```

## LICENSE

This project is under [BSD-3-Clause](LICENSE).

52Poké (神奇宝贝部落格/神奇寶貝部落格, 神奇宝贝百科/神奇寶貝百科) is a Chinese-language Pokémon fan site. Neither the name of 52Poké nor the names of the contributors may be used to endorse any usage of codes under this project.

Pokémon ©2020 Pokémon. ©1995-2020 Nintendo/Creatures Inc./GAME FREAK inc. 52Poké and this project is not affiliated with any Pokémon-related companies.

---

<a name="why-eventbus">1</a>: `Special:RunSingleJob` verifies `mediawiki_signature` in event data. However the order of keys in event structure may not be preserved, thus results in invalid signature. This patch sorts keys before generating and verifying job signature.