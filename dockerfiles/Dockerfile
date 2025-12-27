FROM --platform=$BUILDPLATFORM alpine:3.23.2 as alpine

RUN apk add --no-cache \
    ca-certificates \
    tzdata

FROM scratch
# Note: This label uses the original project namespace (com.centurylinklabs.watchtower)
# for backward compatibility. Changing it would break existing user configurations
# and scripts that rely on this label to identify watchtower containers.
LABEL "com.centurylinklabs.watchtower"="true"

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
