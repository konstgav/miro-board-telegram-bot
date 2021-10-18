FROM golang:1.16
WORKDIR /app
COPY . .
RUN go mod download
EXPOSE 7000
CMD go run main.go 2>> logfile
