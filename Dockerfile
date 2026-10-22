FROM node:24.22.1-alpine AS builderNodeJS
WORKDIR /app
COPY ui/package.json ui/package-lock.json ./
RUN npm ci
COPY ui/ ./
RUN npm run lint && npm test && npm run build

FROM golang:1.27.3-alpine AS builderGolang
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=builderNodeJS /app/dist ./embed/ui
RUN CGO_ENABLED=0 go build -trimpath -o /out/als .

FROM alpine:3 AS builderEnv
WORKDIR /app
COPY scripts/install-speedtest.sh ./
RUN sh /app/install-speedtest.sh
RUN apk add --no-cache iperf iperf3
RUN rm -rf /app

FROM alpine:3
LABEL maintainer="samlm0 <update@ifdream.net>"
COPY --from=builderEnv / /
COPY --from=builderGolang --chmod=755 /out/als /bin/als

CMD ["/bin/als"]
