FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /server ./cmd/server
FROM alpine:3.22
RUN addgroup -S app && adduser -S -G app app && mkdir /data && chown app:app /data
COPY --from=build /server /server
USER app
ENV ADDR=0.0.0.0:8080 DATA_FILE=/data/tasks.json
EXPOSE 8080
ENTRYPOINT ["/server"]
