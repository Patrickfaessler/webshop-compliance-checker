# Flutter web app served by nginx. Empty API_BASE = same origin, /api is routed by Traefik (DEP-2).
# Build context: repository root.
ARG FLUTTER_VERSION=3.47.9
FROM ghcr.io/cirruslabs/flutter:${FLUTTER_VERSION} AS build
WORKDIR /app
COPY frontend/pubspec.* ./
RUN flutter pub get
COPY frontend/ ./
RUN flutter build web --release --dart-define=API_BASE=

FROM nginx:1.27-alpine
COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/build/web /usr/share/nginx/html
