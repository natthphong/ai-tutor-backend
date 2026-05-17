FROM alpine:3.20
RUN apk --no-cache add tzdata curl coreutils
ENV TZ=Asia/Bangkok
WORKDIR /app
COPY ./goapp ./goapp
CMD ["./goapp"]
