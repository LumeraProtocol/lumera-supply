# syntax=docker/dockerfile:1

FROM golang:1.22 AS build
WORKDIR /src
COPY . .
ARG GIT_TAG
ARG GIT_COMMIT
RUN CGO_ENABLED=0 go build \
  -ldflags "-s -w \
    -X 'main.GitTag=${GIT_TAG}' \
    -X 'main.GitCommit=${GIT_COMMIT}'" \
  -o /out/lumera-supply ./cmd/lumera-supply

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/lumera-supply /usr/local/bin/lumera-supply
COPY policy.json /etc/lumera/policy.json
ENV LUMERA_POLICY_PATH=/etc/lumera/policy.json
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/lumera-supply"]
