#!/bin/sh

/bin/sed -i s#APIURL#$APIURL#g /usr/share/nginx/html/config.js
/usr/sbin/nginx
/usr/local/bin/nextlist