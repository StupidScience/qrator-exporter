FROM alpine:3.9

EXPOSE 9501

RUN apk add --no-cache ca-certificates

COPY qrator-exporter ./

ENTRYPOINT ["./qrator-exporter"]
