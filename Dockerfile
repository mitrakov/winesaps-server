FROM golang:1.10

WORKDIR /go/src

ENV path mitrakov.ru/home/winesaps
COPY ${path} ${path}
COPY ${path}/settings.ini settings.ini
COPY ${path}/levels/ levels/

RUN go get "github.com/go-sql-driver/mysql"
RUN go get "github.com/vaughan0/go-ini"
RUN go get "golang.org/x/crypto/scrypt"

RUN go build -race ${path}

EXPOSE 33996/udp

ENTRYPOINT ["./winesaps"]
