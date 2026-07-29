#!/bin/bash
set -e
echo "=== POST todo 1 ==="
curl -s -X POST http://node.test:3000/api/todos -H 'Content-Type: application/json' -d '{"title":"learn-sb-lxc"}'
echo
echo "=== POST todo 2 ==="
curl -s -X POST http://node.test:3000/api/todos -H 'Content-Type: application/json' -d '{"title":"multi-stage-build"}'
echo
echo "=== GET list ==="
curl -s http://node.test:3000/api/todos
echo
echo "=== PUT mark done id=1 ==="
curl -s -X PUT http://node.test:3000/api/todos/1/done
echo
echo "=== GET list ==="
curl -s http://node.test:3000/api/todos
echo
echo "=== DELETE id=2 ==="
curl -s -o /dev/null -w 'DELETE status: %{http_code}\n' -X DELETE http://node.test:3000/api/todos/2
echo "=== GET final ==="
curl -s http://node.test:3000/api/todos
echo
echo "=== restart ==="
incus restart node-todo
sleep 8
curl -s http://node.test:3000/health
echo
