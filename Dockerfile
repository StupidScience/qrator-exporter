FROM alpine:3.9

EXPOSE 9502

RUN apk add --no-cache ca-certificates

COPY qrator-exporter ./

ENTRYPOINT ["./qrator-exporter"]
