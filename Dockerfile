FROM bitnami/git:2.38.1 as code
WORKDIR /code
RUN git clone https://github.com/lixiaofei123/nextlist_web.git

FROM node:14 AS web_build
WORKDIR /build
COPY --from=code /code/nextlist_web/. .
RUN npm install
RUN npm run build


FROM golang AS build
WORKDIR /build
COPY . .
ENV GOPROXY https://goproxy.io,direct
ENV CGO_ENABLED=0
RUN go build -o nextlist


FROM nginx
COPY --from=web_build /build/dist/ /usr/share/nginx/html/
COPY --from=build /build/nextlist /usr/local/bin/nextlist
RUN ls /usr/share/nginx/html
ENV TZ=Asia/Shanghai
COPY docker-entrypoint.sh /
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
ENTRYPOINT ["sh","/docker-entrypoint.sh"]