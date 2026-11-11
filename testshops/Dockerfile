# The three generated test shops (T-3), one per subdomain. Build context: repository root.
FROM nginx:1.27-alpine
COPY testshops/nginx.conf /etc/nginx/conf.d/default.conf
COPY testshops/site /usr/share/nginx/shops
