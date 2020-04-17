FROM golang:1.14 as builder

WORKDIR /workspace

COPY . .

RUN make build

# Actual container
FROM alpine:3.11

COPY --from=builder /workspace/ferrum /opt/ferrum

# Expose HTTP API port
EXPOSE 80

# Run the server app and listen on port 80
CMD ["/opt/ferrum"]