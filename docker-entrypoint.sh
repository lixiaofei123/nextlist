#!/bin/sh

/bin/sed -i s#http://192.168.144.42:8081#/#g /usr/share/nginx/html/config.js
/usr/sbin/nginx
/usr/local/bin/nextlist