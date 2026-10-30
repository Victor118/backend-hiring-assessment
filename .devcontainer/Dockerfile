FROM mcr.microsoft.com/devcontainers/go:1.26-trixie

ARG GOLANGCI_LINT_VERSION=v2.15.0
ARG GOVULNCHECK_VERSION=v1.9.0

# psql to inspect the database (trixie ships client 17, compatible with the Postgres 16 server).
RUN apt-get update \
    && apt-get install -y --no-install-recommends postgresql-client \
    && rm -rf /var/lib/apt/lists/*

RUN curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b /usr/local/bin "${GOLANGCI_LINT_VERSION}"

# Throwaway GOPATH so root does not leave files in the vscode user's module cache.
RUN GOPATH=/tmp/gopath GOBIN=/usr/local/bin go install "golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}" \
    && rm -rf /tmp/gopath /root/.cache/go-build
