# Build: mise run app:image   (or: podman build -t dutyplan .)
# Run:   podman run --rm -p 8080:8080 -v dutyplan-data:/data dutyplan
# CI publishes it for amd64 and arm64 as ghcr.io/gunnaringe/dutyplan (.github/workflows/image.yml).
#
# One image with the server and the planner. The database is /data/dutyplan.db; /data must be writable by uid 10001
# (SQLite keeps its -wal and -shm files next to the database). There is no login: read «No login» in README.md first.

# The Go build runs on the build machine and cross-compiles (no emulation for arm64).
FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.27.4-trixie AS app
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY web ./web
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/dutyplan ./cmd/dutyplan

# The planner in a virtual environment at /opt/solver, from uv.lock. Not editable, so the venv holds the package itself.
# The same Python image as the final stage: the venv points at its /usr/local/bin/python3.12.
FROM docker.io/library/python:3.12.15-slim-trixie AS solver
COPY --from=ghcr.io/astral-sh/uv:0.13.5 /uv /usr/local/bin/uv
ENV UV_PYTHON_DOWNLOADS=never UV_PYTHON=/usr/local/bin/python3.12 UV_PROJECT_ENVIRONMENT=/opt/solver \
    UV_COMPILE_BYTECODE=1 UV_LINK_MODE=copy
WORKDIR /src/solver
COPY solver/pyproject.toml solver/uv.lock ./
RUN uv sync --locked --no-dev --no-editable --no-install-project
COPY solver/dutyplan_solver ./dutyplan_solver
RUN uv sync --locked --no-dev --no-editable

FROM docker.io/library/python:3.12.15-slim-trixie
COPY --from=solver /opt/solver /opt/solver
COPY --from=app /out/dutyplan /usr/local/bin/dutyplan
# The planner and OR-Tools load from the venv alone (not from the source tree, which is not in the image).
RUN cd / && /opt/solver/bin/python -c "import dutyplan_solver, ortools.sat.python.cp_model" \
    && install -d -o 10001 -g 10001 /data
ENV DUTYPLAN_SOLVER="/opt/solver/bin/python -m dutyplan_solver"
USER 10001:10001
WORKDIR /data
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/dutyplan"]
CMD ["--addr", "0.0.0.0:8080", "--db", "/data/dutyplan.db"]
