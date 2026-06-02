# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26.2
ARG OIC_VERSION=19.28

# ---- Build stage ----
FROM oraclelinux:8 AS builder
ARG GO_VERSION
ARG OIC_VERSION
ARG TARGETARCH

RUN dnf install -y gcc wget tar && dnf clean all

# Install Go (arm64/amd64 auto-detected via TARGETARCH)
RUN wget -q https://go.dev/dl/go${GO_VERSION}.linux-${TARGETARCH}.tar.gz \
    && tar -C /usr/local -xzf go${GO_VERSION}.linux-${TARGETARCH}.tar.gz \
    && rm go${GO_VERSION}.linux-${TARGETARCH}.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"

# Configure Oracle Instant Client repository and install
RUN printf '[ol8_oracle_instantclient]\nname=Oracle Linux 8 Oracle Instant Client\nbaseurl=https://yum.oracle.com/repo/OracleLinux/OL8/oracle/instantclient/$basearch/\ngpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-oracle\ngpgcheck=1\nenabled=1\n' \
    > /etc/yum.repos.d/oracle-instantclient.repo \
    && dnf install -y oracle-instantclient${OIC_VERSION}-basic oracle-instantclient${OIC_VERSION}-devel \
    && dnf clean all

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o chcsvgo .

# ---- Runtime stage ----
FROM oraclelinux:8-slim AS runtime

# Instant Client 19 needs libaio and libnsl at runtime
RUN microdnf install -y libaio libnsl && microdnf clean all

# Copy Oracle Instant Client libraries from build stage
COPY --from=builder /usr/lib/oracle /usr/lib/oracle
RUN IC_VER=$(ls /usr/lib/oracle) \
    && echo "/usr/lib/oracle/${IC_VER}/client64/lib" > /etc/ld.so.conf.d/oracle-ic.conf \
    && ldconfig

COPY --from=builder /src/chcsvgo /usr/local/bin/chcsvgo

ENTRYPOINT ["chcsvgo"]
