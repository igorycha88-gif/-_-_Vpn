FROM node:20-alpine AS build

WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm ci

COPY . .
RUN npm run build

FROM nginx:1.27-alpine

COPY --from=build /app/dist /usr/share/nginx/html
COPY 404.html /usr/share/nginx/html/404.html

COPY <<'EOF' /etc/nginx/conf.d/default.conf
server {
    listen 80;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;

    add_header X-Content-Type-Options nosniff;
    add_header X-Frame-Options SAMEORIGIN;
    add_header X-XSS-Protection "1; mode=block";

    gzip on;
    gzip_types text/plain text/css application/javascript text/xml application/xml;

    location / {
        try_files $uri $uri/ /404.html;
    }

    error_page 404 /404.html;
    location = /404.html {
        internal;
    }
}
EOF

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
