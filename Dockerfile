FROM golang:alpine AS build-env
RUN apk --no-cache add build-base git bzr mercurial gcc

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o main .

# final stage
FROM alpine

LABEL maintainer="OPSXCQ <opsxcq@strm.sh>"
WORKDIR /app
COPY --from=build-env /app/main /app/

EXPOSE 9900
ENTRYPOINT ./main
