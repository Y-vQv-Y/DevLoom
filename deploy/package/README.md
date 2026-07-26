# DevLoom Independent Offline Package Builder

This directory builds an auditable offline package without reading or invoking
the `monkeycode-offline-linux-amd64` directory or its closed installer binary.

```bash
cp deploy/package/package.env.example deploy/package/package.env
# Pin the four infrastructure images; application/runtime images build from source:
bash deploy/package/build.sh --version v1.0.0
```

The build host must be Linux amd64 with Docker Buildx, Node.js, pnpm, Python 3,
tar, gzip, curl, and sha256sum. Every image named in `package.env` must already
exist in the local Docker daemon. The generated archive is written to
`deploy/out/devloom-offline-linux-amd64.tgz` by default.

See `docs/OFFLINE_PACKAGE_BUILD_CN.md` for the package contract, runtime
dependency boundary, branding procedure, installation, upgrade, and audit
details.
