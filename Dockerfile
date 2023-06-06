FROM ubuntu:22.04
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY mii-lama.bin .
COPY config.sample.toml .
ENTRYPOINT [ "./mii-lama.bin" ]
CMD ["--config", "config.sample.toml"]
