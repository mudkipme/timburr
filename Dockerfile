FROM ubuntu:20.04 as builder

WORKDIR /app

ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y ca-certificates golang-go librdkafka-dev

COPY . .
RUN GOOS=linux go build -a -o timburr .

FROM ubuntu:20.04

WORKDIR /app

ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y ca-certificates librdkafka1 && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/timburr /app

ENTRYPOINT ["/app/timburr"]
