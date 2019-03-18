FROM golang:1.10.3-alpine as build-env
WORKDIR /go/src/github.com/dynamicguy/goddd/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o goapp ./cmd/shippingsvc

FROM alpine:3.7
WORKDIR /app
COPY --from=build-env /go/src/github.com/dynamicguy/goddd/booking/docs ./booking/docs
COPY --from=build-env /go/src/github.com/dynamicguy/goddd/tracking/docs ./tracking/docs
COPY --from=build-env /go/src/github.com/dynamicguy/goddd/handling/docs ./handling/docs
COPY --from=build-env /go/src/github.com/dynamicguy/goddd/goapp .
EXPOSE 3000
ENTRYPOINT ["./goapp"]
