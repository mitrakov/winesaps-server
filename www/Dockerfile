FROM nginx:1.18
ENV path /usr/share/nginx/html
COPY img/       ${path}/img/
COPY dist/      ${path}/dist/
COPY index.html ${path}/
COPY styles.css ${path}/
EXPOSE 80
