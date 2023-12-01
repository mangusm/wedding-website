FROM golang:1.22.0 as builder
WORKDIR /app
COPY go.mod go.sum main.go ./
RUN CGO_ENABLED=0 go build -o ./wedding-website -a -ldflags '-extldflags "-static"' . 

FROM scratch
WORKDIR /app/static
COPY static/ .
WORKDIR /app
COPY --from=builder /app/wedding-website .
EXPOSE 8080
EXPOSE 5432
CMD ["/app/wedding-website"]
