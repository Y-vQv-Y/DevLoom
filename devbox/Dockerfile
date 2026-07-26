FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive \
    LANG=C.UTF-8 \
    LC_ALL=C.UTF-8 \
    TZ=Asia/Shanghai

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       bash build-essential ca-certificates curl git jq openssh-client \
       python3 python3-pip python3-venv nodejs npm ripgrep tini unzip zip \
    && npm install --global pnpm \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace
ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["sh", "-lc", "trap : TERM INT; while :; do sleep 3600; done"]
