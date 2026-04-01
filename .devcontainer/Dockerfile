FROM mcr.microsoft.com/devcontainers/go:1-1.24-bookworm

# Remove stale yarn apt source whose GPG key has expired, preventing apt-get
# update from succeeding inside the devcontainer feature install scripts.
RUN rm -f /etc/apt/sources.list.d/yarn.list \
    && apt-get clean
