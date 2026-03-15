FROM golang:1.23-bookworm AS build

WORKDIR /app

COPY . .

RUN go test ./...
RUN go build ./...
RUN mkdir -p /out && go build -buildmode=plugin -o /out/logcheck.so ./plugin

FROM golang:1.23-bookworm

WORKDIR /workspace

COPY --from=build /out/logcheck.so /usr/local/lib/logcheck.so
COPY . /workspace

CMD ["bash"]