FROM golang:alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o goapp .

FROM alpine:latest  
WORKDIR /root/
COPY --from=builder /app/goapp .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/flag .
EXPOSE 8080
CMD ["./goapp"]
