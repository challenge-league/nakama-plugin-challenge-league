FROM golang:1.14.3-buster as builder

ENV GOOS linux
ENV GOARCH amd64
ENV CGO_ENABLED 1

RUN apt-get update && \
    apt-get -y upgrade && \
    apt-get install -y --no-install-recommends ca-certificates gcc libc6-dev git
    # && \
    #git config --global advice.detachedHead false && \
    #git clone --quiet --no-checkout https://github.com/heroiclabs/nakama /go/build/nakama

WORKDIR /go/build/nakama-plugin
COPY --from=localhost:32000/nakama-go:dkozlov /go/build/nakama-go nakama-go
COPY --from=localhost:32000/nakama-apigrpc:dkozlov /go/build/nakama-apigrpc apigrpc
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN bash build.sh
