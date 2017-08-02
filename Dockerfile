FROM alpine:3.5
MAINTAINER "a.burtsev@sdventures.com"

ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH
ENV SRC $GOPATH/src/git.dev/developers/eac

ADD ./ $SRC

WORKDIR $SRC

RUN apk add --no-cache musl-dev go \
    && go install \
    && apk del go musl-dev \
    && rm -rf $GOPATH/src/ \
    && rm -rf $GOPATH/pkg/

ENTRYPOINT ["eac"]