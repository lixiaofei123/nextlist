#!/bin/sh

/bin/sed -i s#APIURL#/backend/#g /usr/share/nginx/html/config.js
/usr/sbin/nginx
/usr/local/bin/nextlist