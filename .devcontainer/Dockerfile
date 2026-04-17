FROM mcr.microsoft.com/devcontainers/go:1-1.24-bookworm

# Remove stale yarn apt source whose GPG key has expired, preventing apt-get
# update from succeeding inside the devcontainer feature install scripts.
RUN rm -f /etc/apt/sources.list.d/yarn.list \
    && apt-get clean

# Persist bash history.
RUN mkdir /commandhistory \
    && touch /commandhistory/.bash_history \
    && chown -R vscode /commandhistory \
    && echo "export PROMPT_COMMAND='history -a' && export HISTFILE=/commandhistory/.bash_history" >> /home/vscode/.bashrc

# Create .claude config directory so the volume mount has correct ownership.
RUN mkdir -p /home/vscode/.claude \
    && chown -R vscode:vscode /home/vscode/.claude

# Add ~/.local/bin to PATH for user-installed binaries (e.g. Claude).
RUN echo 'export PATH="$HOME/.local/bin:$PATH"' >> /home/vscode/.bashrc
