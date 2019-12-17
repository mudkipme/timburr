FROM ubuntu:18.04 as builder

WORKDIR /app

RUN apt-get update && apt-get install -y software-properties-common wget git
RUN add-apt-repository -y ppa:longsleep/golang-backports && apt-get update && apt-get install -y golang-go

RUN wget -qO - https://packages.confluent.io/deb/5.3/archive.key | apt-key add -
RUN add-apt-repository -y "deb [arch=amd64] https://packages.confluent.io/deb/5.3 stable main" && \
    apt-get update && apt-get install -y librdkafka-dev

COPY . .
RUN GOOS=linux go build -a -o timburr .

FROM ubuntu:18.04

WORKDIR /app

RUN apt-get update && apt-get install -y software-properties-common wget
RUN wget -qO - https://packages.confluent.io/deb/5.3/archive.key | apt-key add -
RUN add-apt-repository "deb [arch=amd64] https://packages.confluent.io/deb/5.3 stable main" && \
    apt-get update && apt-get install -y librdkafka1 && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/timburr /app

ENTRYPOINT ["/app/timburr"]
