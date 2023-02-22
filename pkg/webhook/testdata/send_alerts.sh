#!/bin/bash
curl -XPOST -d @alert.json http://127.0.0.1:19093/api/v2/alerts
# curl -XPOST -d @./alert.json http://notification-manager-svc.d3os-monitoring-system.svc:19093/api/v2/alerts