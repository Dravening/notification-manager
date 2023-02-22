# Copyright 2018 The D3os Authors. All rights reserved.
# Use of this source code is governed by a Apache license
# that can be found in the LICENSE file.

# Copyright 2018 The D3os Authors. All rights reserved.
# Use of this source code is governed by a Apache license
# that can be found in the LICENSE file.

FROM golang:1.13 as socket-server

COPY / /
WORKDIR /
ENV GOPROXY=https://goproxy.io
RUN CGO_ENABLED=0 GO111MODULE=on go build -i -ldflags '-w -s' -o socket-server main.go

FROM alpine:3.9

COPY --from=socket-server /socket-server /usr/local/bin/

RUN apk add --update ca-certificates && update-ca-certificates
RUN apk add curl
RUN adduser -D -g d3os -u 1002 d3os
RUN chown -R d3os:d3os /usr/local/bin/socket-server
RUN apk add libcap
RUN setcap 'CAP_NET_BIND_SERVICE=+ep' /usr/local/bin/socket-server

USER d3os
CMD ["sh"]
