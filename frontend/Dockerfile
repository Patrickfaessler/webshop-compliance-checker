# Flutter web app served by nginx. Empty API_BASE = same origin, /api is routed by Traefik (DEP-2).
# Build context: repository root. Flutter comes from the official repository at the exact version used locally
# (third-party Flutter images lag behind the stable releases).
FROM debian:bookworm-slim AS build
RUN apt-get update \
 && apt-get install -y --no-install-recommends git curl unzip xz-utils ca-certificates \
 && rm -rf /var/lib/apt/lists/*
ARG FLUTTER_VERSION=3.47.9
RUN git clone --quiet --depth 1 --branch ${FLUTTER_VERSION} https://github.com/flutter/flutter.git /opt/flutter
ENV PATH=/opt/flutter/bin:$PATH
RUN flutter config --no-analytics --no-cli-animations >/dev/null && flutter precache --web
WORKDIR /app
COPY frontend/pubspec.* ./
RUN flutter pub get
COPY frontend/ ./
RUN flutter build web --release --dart-define=API_BASE=

FROM nginx:1.27-alpine
COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/build/web /usr/share/nginx/html
