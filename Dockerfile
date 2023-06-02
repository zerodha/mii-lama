FROM ubuntu:22.04
WORKDIR /app
COPY mii-lama.bin .
COPY config.sample.toml .
CMD ["./mii-lama.bin"]
