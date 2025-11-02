FROM golang:1.22-alpine AS build

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/server ./...

FROM gcr.io/distroless/base-debian12:nonroot

ENV HTTP_ADDR=:8080
EXPOSE 8080

COPY --from=build /bin/server /bin/server

ENTRYPOINT ["/bin/server"]
