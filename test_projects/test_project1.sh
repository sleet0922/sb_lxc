#!/bin/bash
# 测试项目 1: go_fullstack

GO_IP=$(incus list go-fullstack --format csv -c 4 2>/dev/null | head -1)
GO_URL="http://${GO_IP}:8080"
echo "Go URL: $GO_URL"

echo "--- 1.1 Health ---"
R=$(curl -s --connect-timeout 5 "$GO_URL/health")
echo "  Result: $R"
echo "$R" | grep -q '"ok"' && echo "  PASS" || echo "  FAIL"

echo "--- 1.2 Create Todo ---"
R=$(curl -s --connect-timeout 5 -X POST "$GO_URL/api/todos" -H "Content-Type: application/json" -d '{"title":"learn-sb-lxc"}')
echo "  Result: $R"
echo "$R" | grep -q '"id"' && echo "  PASS" || echo "  FAIL"

echo "--- 1.3 Create Second ---"
R=$(curl -s --connect-timeout 5 -X POST "$GO_URL/api/todos" -H "Content-Type: application/json" -d '{"title":"build-app"}')
echo "  Result: $R"
echo "$R" | grep -q '"id"' && echo "  PASS" || echo "  FAIL"

echo "--- 1.4 List ---"
R=$(curl -s --connect-timeout 5 "$GO_URL/api/todos")
echo "  Result: $R"
echo "$R" | grep -q 'learn-sb-lxc' && echo "  PASS" || echo "  FAIL"

echo "--- 1.5 Mark Done ---"
ID=$(echo "$R" | grep -o '"id":[0-9]*' | head -1 | grep -o '[0-9]*')
R=$(curl -s --connect-timeout 5 -X PUT "$GO_URL/api/todos/$ID/done" -H "Content-Type: application/json" -d '{"done":true}')
echo "  Result: $R"
echo "$R" | grep -q '"done":true' && echo "  PASS" || echo "  FAIL"

echo "--- 1.6 Frontend ---"
R=$(curl -s --connect-timeout 5 "$GO_URL/")
echo "  Length: ${#R}"
echo "$R" | grep -q '<html' && echo "  PASS" || echo "  FAIL"

echo "--- Project 1 Done ---"
