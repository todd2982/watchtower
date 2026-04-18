FROM --platform=$BUILDPLATFORM alpine:3.23.4 as alpine

RUN apk add --no-cache \
    ca-certificates \
    tzdata

FROM scratch
# Note: This label uses the original project namespace (com.centurylinklabs.watchtower)
# for backward compatibility. Changing it would break existing user configurations
# and scripts that rely on this label to identify watchtower containers.
LABEL "com.centurylinklabs.watchtower"="true"
LABEL org.opencontainers.image.source="https://github.com/todd2982/watchtower"
LABEL org.opencontainers.image.documentation="https://todd2982.dev/watchtower"

COPY --from=alpine \
    /etc/ssl/certs/ca-certificates.crt \
    /etc/ssl/certs/ca-certificates.crt
COPY --from=alpine \
    /usr/share/zoneinfo \
    /usr/share/zoneinfo

EXPOSE 8080

COPY watchtower /

HEALTHCHECK CMD [ "/watchtower", "--health-check"]

ENTRYPOINT ["/watchtower"]
