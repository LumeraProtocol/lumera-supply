# syntax=docker/dockerfile:1

FROM golang:1.22 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/lumera-supply ./cmd/lumera-supply

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/lumera-supply /usr/local/bin/lumera-supply
COPY policy.json /etc/lumera/policy.json
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/lumera-supply"]
CMD ["-addr=:8080", "-lcd=http://localhost:1317", "-policy=/etc/lumera/policy.json", "-denom=ulume"]
