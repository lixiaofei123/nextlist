FROM golang AS build
WORKDIR /build
COPY . .
ENV GOPROXY https://goproxy.io,direct
ENV CGO_ENABLED=0
RUN go build -o nextlist

FROM alpine:3.9
RUN apk add --no-cache ca-certificates
COPY --from=build /build/nextlist /usr/local/bin/nextlist
RUN apk add -U tzdata
EXPOSE 8081
ENTRYPOINT ["nextlist"]
CMD ["-c","./config.yaml"]