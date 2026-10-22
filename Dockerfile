FROM node:24.22.1-alpine AS builderNodeJS
WORKDIR /app
COPY ui/package.json ui/package-lock.json ./
RUN npm ci
COPY ui/ ./
RUN npm run lint && npm test && npm run build

FROM golang:1.27.3-alpine AS builderGolang
RUN apk add --no-cache ca-certificates patch wget
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY patches/nexttrace-remove-file-output.patch /app/patches/nexttrace-remove-file-output.patch
COPY scripts/build-nexttrace.sh /app/scripts/build-nexttrace.sh
COPY --from=builderNodeJS /app/dist ./embed/ui
RUN CGO_ENABLED=0 go build -trimpath -o /out/als .
RUN CGO_ENABLED=0 /app/scripts/build-nexttrace.sh /out/nexttrace

FROM alpine:3 AS builderEnv
WORKDIR /app
COPY scripts/install-speedtest.sh ./
RUN sh /app/install-speedtest.sh
RUN apk add --no-cache \
    iperf iperf3 \
    mtr \
    traceroute \
    iputils
RUN rm -rf /app

FROM alpine:3
LABEL maintainer="samlm0 <update@ifdream.net>"
COPY --from=builderEnv / /
COPY --from=builderGolang --chmod=755 /out/als /bin/als
COPY --from=builderGolang --chmod=755 /out/nexttrace /usr/local/bin/nexttrace

CMD ["/bin/als"]
